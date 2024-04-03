package cip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/diwise/context-broker/pkg/datamodels/fiware"
	"github.com/diwise/context-broker/pkg/ngsild/client"
	ngsierrors "github.com/diwise/context-broker/pkg/ngsild/errors"
	"github.com/diwise/context-broker/pkg/ngsild/types/entities"
	"github.com/diwise/context-broker/pkg/ngsild/types/entities/decorators"
	"github.com/diwise/context-broker/pkg/ngsild/types/properties"
	"github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func CreateWaterQualityObserved(ctx context.Context, temperatures []models.Temperature, cbClient client.ContextBrokerClient) error {
	logger := logging.GetFromContext(ctx)

	for _, temp := range temperatures {
		err := createOrMergeTemperature(ctx, temp, cbClient)
		if err != nil {
			logger.Error("failed to create/merge entity", "err", err.Error())
			return err
		}
	}

	return nil
}

func temperature(temp float64, observedAt time.Time) entities.EntityDecoratorFunc {
	ts := observedAt.UTC().Format(time.RFC3339Nano)
	return decorators.Number("temperature", temp, properties.ObservedAt(ts))
}

func createOrMergeTemperature(ctx context.Context, temp models.Temperature, cbClient client.ContextBrokerClient) error {
	headers := map[string][]string{"Content-Type": {"application/ld+json"}}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	properties := []entities.EntityDecoratorFunc{
		entities.DefaultContext(),
		decorators.Location(temp.Lat, temp.Lon),
		decorators.DateObserved(temp.Date.UTC().Format(time.RFC3339Nano)),
		temperature(temp.Temp, temp.Date.UTC()),
		decorators.Text("source", temp.Source),
	}

	id := fiware.WaterQualityObservedIDPrefix + temp.InternalID

	fragment, err := entities.NewFragment(properties...)
	if err != nil {
		return fmt.Errorf("unable to create new fragment for %s, %w", id, err)
	}

	_, err = cbClient.MergeEntity(ctxWithTimeout, id, fragment, headers)

	if err != nil {
		if !errors.Is(err, ngsierrors.ErrNotFound) {
			return fmt.Errorf("unable to merge entity %s, %w", id, err)
		}

		entity, err := entities.New(id, fiware.WaterQualityObservedTypeName, properties...)
		if err != nil {
			return fmt.Errorf("unable to create new entity %s, %w", id, err)
		}

		_, err = cbClient.CreateEntity(ctxWithTimeout, entity, headers)
		if err != nil {
			return err
		}
	}

	return nil
}
