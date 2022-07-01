package havochvatten

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("integration-cip-havochvatten")

type HovClient interface {
	Details(ctx context.Context) ([]Detail, error)
	Detail(ctx context.Context, nutsCode string) (*Detail, error)
	DetailWithTestResults(ctx context.Context, nutsCode string) (*DetailWithTestResults, error)
	BathWaterProfile(ctx context.Context, nutsCode string) (*BathWaterProfile, error)
	ApiUrl() string
}

type hovClient struct {
	apiUrl string
}

func (h hovClient) ApiUrl() string {
	return h.apiUrl
}

type Detail struct {
	NutsCode    string  `json:"nutsCode"`
	Name        string  `json:"locationName"`
	Area        string  `json:"locationArea"`
	Description string  `json:"bathInformation"`
	SampleDate  *int64  `json:"sampleDate"`
	Temperature *string `json:"sampleTemperature"`
}

func (d Detail) Date() time.Time {
	if d.Temperature == nil {
		return time.Time{}
	}
	return time.Unix(*d.SampleDate/1000, 0)
}

type TestResult struct {
	//Provtagningsdatum
	SampleDate time.Time `json:"sampleDate"`
	//Vattentemperatur, enhet: °C
	TempValue string `json:"tempValue"`
}

type DetailWithTestResults struct {
	Detail
	TestResults []TestResult `json:"testResult"`
}

type BathWaterProfile struct {
	Name                string      `json:"name"`
	Description         string      `json:"description"`
	NutsCode            string      `json:"nutsCode"`
	ProfileLatestUpdate int64       `json:"profileLatestUpdate"`
	CoperSmhi           []CoperSmhi `json:"coperSmhi"`
	Lat                 float64     `json:"decLat"`
	Long                float64     `json:"decLong"`
}

type CoperSmhi struct {
	//Vattentemperatur för den aktuella tidpunkten, angiven i measHour
	CopernicusData string `json:"copernicusData"`
	//Lufttemperatur för den aktuella tidpunkten, angiven i measHour
	SmhiTemp string `json:"smhiTemp"`
	//Vindstyrka enligt SMHI vid aktuell tidpunkt, angiven i measHour
	SmhiWs string `json:"smhiWs"`
	//Medelnederbörd per timme [mm/h] vid aktuell tidpunkt, angiven i measHour
	SmhiWsymb string `json:"smhiWsymb"`
	//Vädersituation angivet som ett tal mellan 1 - 27 vid aktuell tidpunkt, angiven i measHour
	SmhiPmean string `json:"smhiPmean"`
	//Vindriktning (0 – 359°) enligt SMHI vid aktuell tidpunkt, angiven i measHour
	SmhiWindDir string `json:"smhiWindDir"`
	//Klockslag som prognosen avser, endast timmen på 24h format
	MeasHour string `json:"measHour"`
}

func (c CoperSmhi) Date() (time.Time, bool) {
	if hour, err := strconv.Atoi(c.MeasHour); err == nil {
		now := time.Now()
		dateStr := fmt.Sprintf("%d-%02d-%02dT%d:00:00+02:00", now.Year(), now.Month(), now.Day(), hour)
		m, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			return time.Time{}, false
		}

		return m.UTC(), true
	}
	return time.Time{}, false
}

func New(apiUrl string) HovClient {
	return &hovClient{
		apiUrl: apiUrl,
	}
}

func (h hovClient) Details(ctx context.Context) ([]Detail, error) {
	url := fmt.Sprintf("%s/detail", h.apiUrl)
	b, status, err := get(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, nil
	}

	var details []Detail
	err = json.Unmarshal(b, &details)
	if err != nil {
		return nil, err
	}

	return details, nil
}

func (h hovClient) Detail(ctx context.Context, nutsCode string) (*Detail, error) {
	url := fmt.Sprintf("%s/detail/%s", h.apiUrl, nutsCode)
	b, status, err := get(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, nil
	}

	var detail Detail
	err = json.Unmarshal(b, &detail)
	if err != nil {
		return nil, err
	}

	return &detail, nil
}

func (h hovClient) DetailWithTestResults(ctx context.Context, nutsCode string) (*DetailWithTestResults, error) {
	return nil, nil
}

func (h hovClient) BathWaterProfile(ctx context.Context, nutsCode string) (*BathWaterProfile, error) {
	url := fmt.Sprintf("%s/testlocationprofile/%s", h.apiUrl, nutsCode)
	b, status, err := get(ctx, url)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, nil
	}

	var bathWaterProfile BathWaterProfile
	err = json.Unmarshal(b, &bathWaterProfile)
	if err != nil {
		return nil, err
	}

	return &bathWaterProfile, nil
}

func get(ctx context.Context, url string) ([]byte, int, error) {

	var err error

	ctx, span := tracer.Start(ctx, "integration-cip-havochvatten")
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

	return body, resp.StatusCode, nil
}
