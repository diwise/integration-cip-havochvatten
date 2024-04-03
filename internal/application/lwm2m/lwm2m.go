package lwm2m

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/diwise/senml"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("integration-cip-havochvatten/lwm2m")

const (
	TemperatureURN string = "urn:oma:lwm2m:ext:3303"
)

func CreateTemperatures(ctx context.Context, temperatures []models.Temperature, url string) error {
	logger := logging.GetFromContext(ctx)

	var errs []error

	for _, t := range temperatures {
		log := logger.With(slog.String("nutsCode", t.NutsCode)).With(slog.String("device_id", t.InternalID))

		pack, err := createSenMLPackage(t)
		if err != nil {
			log.Error("unable to create lwm2m temperature object", "err", err.Error())
			continue
		}

		log.Info(fmt.Sprintf("sending lwm2m pack for %s", t.Date.Format(time.RFC3339)))

		err = send(ctx, url, pack)
		if err != nil {
			log.Error("unable to POST lwm2m temperature", "err", err.Error())
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}

func createSenMLPackage(t models.Temperature) (senml.Pack, error) {
	SensorValue := func(v float64, t time.Time) SenMLDecoratorFunc {
		return Value("5700", v, t, senml.UnitCelsius)
	}

	pack := NewSenMLPack(t, SensorValue(t.Temp, t.Date))

	return pack, nil
}

func send(ctx context.Context, url string, pack senml.Pack) error {
	var err error

	ctx, span := tracer.Start(ctx, "send-object")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	httpClient := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	b, err := json.Marshal(pack)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/vnd.oma.lwm2m.3303+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusCreated {
		err = fmt.Errorf("unexpected response code %d", resp.StatusCode)
	}

	return err
}
