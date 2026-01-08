package lwm2m

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	log := logging.GetFromContext(ctx)

	var errs []error

	for _, t := range temperatures {
		pack, err := createSenMLPackage(ctx, t)
		if err != nil {
			log.Error("unable to create lwm2m temperature object", "nutsCode", t.NutsCode, "device_id", t.InternalID, "err", err.Error())
			continue
		}

		log.Info(fmt.Sprintf("sending lwm2m pack for %s", t.Date.Format(time.RFC3339)))

		err = func() error {
			tmoctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			return send(tmoctx, url, pack)
		}()
		if err != nil {
			log.Error("unable to POST lwm2m temperature", "nutsCode", t.NutsCode, "device_id", t.InternalID, "err", err.Error())
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}

func createSenMLPackage(ctx context.Context, t models.Temperature) (senml.Pack, error) {
	log := logging.GetFromContext(ctx)

	log.Debug("creating lwm2m temperature object", "nutsCode", t.NutsCode, "device_id", t.InternalID, "date", t.Date.Format(time.RFC3339), "temp", t.Temp, "source", t.Source)

	SensorValue := func(v float64) SenMLDecoratorFunc {
		return Value("5700", v, senml.UnitCelsius)
	}

	pack := NewSenMLPack(t, SensorValue(t.Temp))

	return pack, nil
}

func send(ctx context.Context, url string, pack senml.Pack) error {
	var err error

	ctx, span := tracer.Start(ctx, "send-object")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	transport := http.DefaultTransport
	if transport == nil {
		transport = &http.Transport{}
	}

	httpClient := http.Client{
		Transport: otelhttp.NewTransport(transport),
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
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		err = fmt.Errorf("unexpected response code %d", resp.StatusCode)
	}

	return err
}
