package lwm2m

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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

func CreateTemperatures(ctx context.Context, temperatures []models.Temperature, url string) error {
	logger := logging.GetFromContext(ctx)

	var errs []error

	for _, t := range temperatures {
		log := logger.With().
			Str("nutsCode", t.NutsCode).
			Str("device_id", t.InternalID).Logger()

		pack, err := temperature(ctx, t.InternalID, t)
		if err != nil {
			log.Error().Err(err).Msg("unable to create lwm2m temperature object")
			continue
		}

		log.Info().Msgf("sending lwm2m pack for %s", t.Date.Format(time.RFC3339))

		err = send(ctx, url, pack)
		if err != nil {
			log.Error().Err(err).Msg("unable to POST lwm2m temperature")
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}

func temperature(ctx context.Context, deviceID string, t models.Temperature) (senml.Pack, error) {
	SensorValue := func(v float64, t time.Time) SenMLDecoratorFunc {
		return Value("5700", v, t, senml.UnitCelsius)
	}

	pack := NewSenMLPack(deviceID, TemperatureURN, t.Date, SensorValue(t.Temp, t.Date))

	return pack, nil
}

func send(ctx context.Context, url string, pack senml.Pack) error {
	var err error

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
		return err
	} else if resp.StatusCode != http.StatusCreated {
		err = fmt.Errorf("unexpected response code %d", resp.StatusCode)
	}

	return err
}
