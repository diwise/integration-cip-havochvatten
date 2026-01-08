package havochvatten

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	Load(ctx context.Context, nutsCodes map[string]string) ([]models.Temperature, error)
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

	transport := http.DefaultTransport
	if transport == nil {
		transport = &http.Transport{}
	}

	httpClient := http.Client{
		Transport: otelhttp.NewTransport(transport),
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

	return body, resp.StatusCode, nil
}

func (h hovClient) Load(ctx context.Context, nutsCodes map[string]string) ([]models.Temperature, error) {
	log := logging.GetFromContext(ctx)

	result := make([]models.Temperature, 0)

	log.Debug(fmt.Sprintf("loading temperature data for %d nuts codes...", len(nutsCodes)))

	count := 0

	for nutsCode, internalID := range nutsCodes {
		count += 1

		logger := log.With(slog.String("nutsCode", nutsCode))

		profile, err := h.BathWaterProfile(ctx, nutsCode)
		if err != nil {
			logger.Error("failed to get BathWaterProfile", "err", err.Error())
			continue
		}

		sampleTemp := false
		coperSmhi := false

		detail, err := h.Detail(ctx, nutsCode)

		if err == nil && detail != nil && detail.Temperature != nil {
			if t, err := strconv.ParseFloat(*detail.Temperature, 64); err == nil {
				sampleTemp = true

				result = append(result, models.Temperature{
					NutsCode:   profile.NutsCode,
					InternalID: internalID,
					Lat:        profile.Lat,
					Lon:        profile.Long,
					Date:       detail.Date(),
					Temp:       t,
					Source:     h.Source(),
				})
			}
		} else {
			logger.Debug("could not fetch sampled temperature", "profile_name", profile.Name)
		}

		soon := time.Now().UTC().Add(5 * time.Minute)

		for _, c := range profile.CoperSmhi {
			if date, ok := c.Date(); ok && c.CopernicusData != "" {
				// Exclude temperature values from the future
				if date.Before(soon) {
					if t, err := strconv.ParseFloat(c.CopernicusData, 64); err == nil {
						coperSmhi = true
						result = append(result, models.Temperature{
							NutsCode:   profile.NutsCode,
							InternalID: internalID,
							Lat:        profile.Lat,
							Lon:        profile.Long,
							Date:       date,
							Temp:       t,
							Source:     "https://www.smhi.se",
						})
					}
				}
			}
		}

		logger.Debug(fmt.Sprintf("temperature [sample: %t, copernicus: %t] for %s (%d) loaded", sampleTemp, coperSmhi, profile.Name, count))

		time.Sleep(500 * time.Millisecond)
	}

	return result, nil
}
