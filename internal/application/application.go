package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/context-broker/pkg/datamodels/fiware"
	"github.com/diwise/context-broker/pkg/ngsild/client"
	ngsierrors "github.com/diwise/context-broker/pkg/ngsild/errors"
	"github.com/diwise/context-broker/pkg/ngsild/types"
	"github.com/diwise/context-broker/pkg/ngsild/types/entities"
	. "github.com/diwise/context-broker/pkg/ngsild/types/entities/decorators"
	"github.com/diwise/integration-cip-havochvatten/internal/application/havochvatten"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

type App interface {
	CreateWaterQualityObserved(ctx context.Context, nutsCodes func() []NutsCode) error
}

type NutsCode string

type app struct {
	h  havochvatten.HovClient
	cb client.ContextBrokerClient
}

func New(h havochvatten.HovClient, cb client.ContextBrokerClient) App {
	return &app{
		h:  h,
		cb: cb,
	}
}

func (a app) CreateWaterQualityObserved(ctx context.Context, nutsCodes func() []NutsCode) error {
	log := logging.GetFromContext(ctx)

	for _, nutsCode := range nutsCodes() {
		log.Info().Msgf("creating wqo entities for beach %s", nutsCode)

		detail, err := a.h.Detail(ctx, string(nutsCode))
		if err != nil {
			log.Error().Err(err).Msg("failed to get details")
			continue
		}

		log.Info().Msgf("%s is %s", nutsCode, detail.Name)

		if detail.Temperature == nil {
			log.Info().Msg("temperature has not been sampled for this beach")
			continue
		}

		profile, err := a.h.BathWaterProfile(ctx, string(nutsCode))
		if err != nil {
			log.Error().Err(err).Msg("failed to get BathWaterProfile")
			continue
		}

		t, err := strconv.ParseFloat(*detail.Temperature, 64)
		if err != nil {
			log.Error().Err(err).Msgf("failed to convert temperature value %s", *detail.Temperature)
			continue
		}

		wqo, err := newWaterQualityObserved(profile.NutsCode, profile.Lat, profile.Long, detail.Date(), Temperature(t), Text("source", a.h.ApiUrl()))
		if err != nil {
			log.Error().Err(err).Msg("failed to construct a new WaterQualityObserved")
			continue
		}

		wqob, _ := json.Marshal(wqo)
		log.Debug().Msgf("creating entity: %s", string(wqob))

		headers := map[string][]string{"Content-Type": {"application/ld+json"}}
		_, err = a.cb.CreateEntity(ctx, wqo, headers)
		if err != nil {
			if !errors.Is(err, ngsierrors.ErrAlreadyExists) {
				log.Error().Err(err).Msg("failed to create wqo entity")
			} else {
				err = nil
				log.Debug().Msg("entity already existed")
			}
		}

		for _, c := range profile.CoperSmhi {
			if date, ok := c.Date(); ok && c.CopernicusData != "" {
				if t, err := strconv.ParseFloat(c.CopernicusData, 64); err == nil {
					wqo, err = newWaterQualityObserved(profile.NutsCode, profile.Lat, profile.Long, date, Temperature(t), Text("source", "https://www.smhi.se/"))
					if err != nil {
						log.Error().Err(err).Msg("could not construct WaterQualityObserved")
						continue
					}

					wqob, _ = json.Marshal(wqo)
					log.Debug().Msgf("creating entity: %s", string(wqob))

					_, err = a.cb.CreateEntity(ctx, wqo, headers)
					if err != nil {
						if !errors.Is(err, ngsierrors.ErrAlreadyExists) {
							log.Error().Err(err).Msg("failed to create wqo entity")
						} else {
							err = nil
							log.Debug().Msg("entity already existed")
						}
					}
				}
			}
		}

		log.Info().Msgf("sleeping to prevent rate limiting ...")
		time.Sleep(2 * time.Second)
	}

	return nil
}

func newWaterQualityObserved(observationID string, latitude float64, longitude float64, observedAt time.Time, decorators ...entities.EntityDecoratorFunc) (types.Entity, error) {
	if len(decorators) == 0 {
		return nil, fmt.Errorf("at least one property must be set in a WaterQualityObserved entity")
	}

	if !strings.HasPrefix(observationID, fiware.WaterQualityObservedIDPrefix) {
		observationID = fiware.WaterQualityObservedIDPrefix + observationID
	}

	const RFC3339WithoutHyphensAndColons string = "20060102T150405Z07:00"
	observationID = observationID + ":" + strings.ReplaceAll(observedAt.Format(RFC3339WithoutHyphensAndColons), " ", "")

	decorators = append(decorators, entities.DefaultContext(), DateObserved(observedAt.Format(time.RFC3339)), Location(latitude, longitude))

	e, err := entities.New(
		observationID, fiware.WaterQualityObservedTypeName,
		decorators...,
	)

	return e, err
}
