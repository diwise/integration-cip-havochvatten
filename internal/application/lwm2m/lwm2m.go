package lwm2m

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"github.com/farshidtz/senml/v2"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

var tlsSkipVerify bool

func init() {
	tlsSkipVerify = env.GetVariableOrDefault(zerolog.Logger{}, "TLS_SKIP_VERIFY", "0") == "1"
}

var tracer = otel.Tracer("integration-cip-havochvatten/lwm2m")

const (
	TemperatureURN string = "urn:oma:lwm2m:ext:3303"
)

func CreateTemperatures(ctx context.Context, temperatures []models.Temperature, iotUrl string) error {
	logger := logging.GetFromContext(ctx)

	for _, t := range temperatures {
		pack, err := temperature(ctx, t.NutsCode, t)
		if err != nil {
			logger.Error().Err(err).Msg("unable to create lwm2m temperature object")
			continue
		}

		err = send(ctx, iotUrl, pack)
		if err != nil {
			logger.Error().Err(err).Msg("unable to send lwm2m object")
			return err
		}
	}

	return nil
}

func temperature(ctx context.Context, deviceID string, t models.Temperature) (senml.Pack, error) {
	SensorValue := func(v float64) SenMLDecoratorFunc { return Value("5700", v) }
	pack := NewSenMLPack(deviceID, TemperatureURN, t.Date, SensorValue(t.Temp))
	return pack, nil
}

func send(ctx context.Context, url string, pack senml.Pack) error {
	var err error

	log := logging.GetFromContext(ctx)

	ctx, span := tracer.Start(ctx, "send-object")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var httpClient http.Client

	if tlsSkipVerify {
		customTransport := http.DefaultTransport.(*http.Transport).Clone()
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		httpClient = http.Client{
			Transport: otelhttp.NewTransport(customTransport),
		}
	} else {
		httpClient = http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
	}

	b, err := json.Marshal(pack)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/senml+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("failed to send lwm2m object")
	} else if resp.StatusCode != http.StatusCreated {
		err = fmt.Errorf("unexpected response code %d", resp.StatusCode)
		log.Error().Err(err).Msg("failed to send lwm2m object")
	}

	return err
}
