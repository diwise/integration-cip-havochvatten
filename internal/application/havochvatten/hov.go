package havochvatten

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("integration-cip-havochvatten/client")

type HovClient interface {
	Details(ctx context.Context) ([]models.Detail, error)
	Detail(ctx context.Context, nutsCode string) (*models.Detail, error)
	DetailWithTestResults(ctx context.Context, nutsCode string) (*models.DetailWithTestResults, error)
	BathWaterProfile(ctx context.Context, nutsCode string) (*models.BathWaterProfile, error)
	Source() string
	Load(ctx context.Context, nutsCodes []models.NutsCode) ([]models.Temperature, error)
}

type hovClient struct {
	apiUrl string
}

func (h hovClient) Source() string {
	return h.apiUrl
}

func New(apiUrl string) HovClient {
	return &hovClient{
		apiUrl: apiUrl,
	}
}

func (h hovClient) Details(ctx context.Context) ([]models.Detail, error) {
	url := fmt.Sprintf("%s/detail", h.apiUrl)
	b, status, err := get(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, nil
	}

	var details []models.Detail
	err = json.Unmarshal(b, &details)
	if err != nil {
		return nil, err
	}

	return details, nil
}

func (h hovClient) Detail(ctx context.Context, nutsCode string) (*models.Detail, error) {
	url := fmt.Sprintf("%s/detail/%s", h.apiUrl, strings.ToUpper(nutsCode))
	b, status, err := get(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, nil
	}

	var detail models.Detail
	err = json.Unmarshal(b, &detail)
	if err != nil {
		return nil, err
	}

	return &detail, nil
}

func (h hovClient) DetailWithTestResults(ctx context.Context, nutsCode string) (*models.DetailWithTestResults, error) {
	return nil, nil
}

func (h hovClient) BathWaterProfile(ctx context.Context, nutsCode string) (*models.BathWaterProfile, error) {
	url := fmt.Sprintf("%s/testlocationprofile/%s", h.apiUrl, strings.ToUpper(nutsCode))
	b, status, err := get(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, nil
	}

	var bathWaterProfile models.BathWaterProfile
	err = json.Unmarshal(b, &bathWaterProfile)
	if err != nil {
		return nil, err
	}

	return &bathWaterProfile, nil
}

func get(ctx context.Context, url string) ([]byte, int, error) {

	var err error

	ctx, span := tracer.Start(ctx, "get-data")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	httpClient := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("request failed: %s", err.Error())
		return nil, http.StatusInternalServerError, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, http.StatusNotFound, nil
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("expectation failed: expected status code %d, but got %d", http.StatusOK, resp.StatusCode)
		return nil, resp.StatusCode, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %s", err.Error())
		return nil, http.StatusInternalServerError, err
	}

	time.Sleep(1 * time.Second)

	return body, resp.StatusCode, nil
}

func (h hovClient) Load(ctx context.Context, nutsCodes []models.NutsCode) ([]models.Temperature, error) {
	log := logging.GetFromContext(ctx)

	result := make([]models.Temperature, 0)

	for _, nutsCode := range nutsCodes {
		logger := log.With().Str("NutsCode", string(nutsCode)).Logger()

		detail, err := h.Detail(ctx, string(nutsCode))
		if err != nil {
			logger.Error().Err(err).Msg("failed to get details")
			continue
		}

		if detail.Temperature == nil {
			logger.Info().Msgf("temperature has not been sampled for this beach %s", detail.Name)
			continue
		}

		profile, err := h.BathWaterProfile(ctx, string(nutsCode))
		if err != nil {
			logger.Error().Err(err).Msg("failed to get BathWaterProfile")
			continue
		}

		t, err := strconv.ParseFloat(*detail.Temperature, 64)
		if err != nil {
			logger.Error().Err(err).Msgf("failed to convert temperature value %s", *detail.Temperature)
			continue
		}

		result = append(result, models.Temperature{
			NutsCode: profile.NutsCode,
			Lat:      profile.Lat,
			Lon:      profile.Long,
			Date:     detail.Date(),
			Temp:     t,
			Source:   h.Source(),
		})

		for _, c := range profile.CoperSmhi {
			if date, ok := c.Date(); ok && c.CopernicusData != "" {
				if t, err := strconv.ParseFloat(c.CopernicusData, 64); err == nil {
					result = append(result, models.Temperature{
						NutsCode: profile.NutsCode,
						Lat:      profile.Lat,
						Lon:      profile.Long,
						Date:     date,
						Temp:     t,
						Source:   "https://www.smhi.se",
					})
				}
			}
		}
	}

	return result, nil
}
