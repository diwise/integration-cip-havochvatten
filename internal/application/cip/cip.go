package transform

import (
	"context"
	"errors"
	"time"

	"github.com/diwise/context-broker/pkg/datamodels/fiware"
	"github.com/diwise/context-broker/pkg/ngsild/client"
	ngsierrors "github.com/diwise/context-broker/pkg/ngsild/errors"
	"github.com/diwise/context-broker/pkg/ngsild/types/entities"
	"github.com/diwise/context-broker/pkg/ngsild/types/entities/decorators"
	"github.com/diwise/context-broker/pkg/ngsild/types/properties"
	model "github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func CreateWaterQualityObserved(ctx context.Context, temperatures []model.Temperature, cbClient client.ContextBrokerClient) error {
	logger := logging.GetFromContext(ctx)

	for _, temp := range temperatures {
		err := createOrMergeTemperature(ctx, temp, cbClient)
		if err != nil {
			logger.Error().Err(err).Msg("failed to create/merge entity")
		}
	}

	return nil
}

func temperature(temp float64, observedAt time.Time) entities.EntityDecoratorFunc {
	ts := observedAt.UTC().Format(time.RFC3339Nano)
	return decorators.Number("temperature", temp, properties.ObservedAt(ts))
}

func createOrMergeTemperature(ctx context.Context, temp model.Temperature, cbClient client.ContextBrokerClient) error {
	logger := logging.GetFromContext(ctx)
	headers := map[string][]string{"Content-Type": {"application/ld+json"}}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	properties := []entities.EntityDecoratorFunc{
		entities.DefaultContext(),
		decorators.Location(temp.Lat, temp.Lon),
		decorators.DateObserved(temp.Date.UTC().Format(time.RFC3339Nano)),
		temperature(temp.Temp, temp.Date),
		decorators.Text("source", temp.Source),
	}

	id := fiware.WaterQualityObservedIDPrefix + "nuts:" + temp.NutsCode

	fragment, err := entities.NewFragment(properties...)
	if err != nil {
		return err
	}

	logger = logger.With().Str("entityID", id).Logger()

	_, err = cbClient.MergeEntity(ctxWithTimeout, id, fragment, headers)

	if err != nil {
		if !errors.Is(err, ngsierrors.ErrNotFound) {
			logger.Error().Err(err).Msg("failed to merge entity")
			return err
		}

		entity, err := entities.New(id, fiware.WaterQualityObservedTypeName, properties...)
		if err != nil {
			return err
		}

		_, err = cbClient.CreateEntity(ctxWithTimeout, entity, headers)
		if err != nil {
			logger.Error().Err(err).Msg("failed to create entity")
			return err
		}
	}

	return nil
}
