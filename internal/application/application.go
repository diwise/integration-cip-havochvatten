package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/diwise/context-broker/pkg/datamodels/fiware"
	"github.com/diwise/context-broker/pkg/ngsild/client"
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
		detail, err := a.h.Detail(ctx, string(nutsCode))
		if err != nil {
			log.Info().Msgf("could not get details for %s because %s", nutsCode, err.Error())
			continue
		}

		profile, err := a.h.BathWaterProfile(ctx, string(nutsCode))
		if err != nil {
			log.Info().Msgf("could not get BathWaterProfile for %s because %s", nutsCode, err.Error())
			continue
		}

		t, err := strconv.ParseFloat(detail.Temperature, 64)
		if err != nil {
			log.Info().Msgf("could not convert temperature for %s because %s", nutsCode, err.Error())
			continue
		}

		wqo, err := newWaterQualityObserved(profile.NutsCode, profile.Lat, profile.Long, detail.Date(), Temperature(t), Text("source", a.h.ApiUrl()))
		if err != nil {
			log.Info().Msgf("could not create WaterQualityObserved for %s because %s", nutsCode, err.Error())
			continue
		}

		_, err = a.cb.CreateEntity(ctx, wqo, map[string][]string{"Content-Type": {"application/ld+json"}})
		if err != nil {
			log.Info().Msgf("could not create entity for %s because %s", nutsCode, err.Error())
		}

		for _, c := range profile.CoperSmhi {
			if date, ok := c.Date(); ok && c.CopernicusData != "" {
				if t, err := strconv.ParseFloat(c.CopernicusData, 64); err == nil {
					wqo, err = newWaterQualityObserved(profile.NutsCode, profile.Lat, profile.Long, date, Temperature(t), Text("source", "https://www.smhi.se/"))
					if err != nil {
						log.Info().Msgf("could not create WaterQualityObserved for %s because %s", nutsCode, err.Error())
						continue
					}
					_, err = a.cb.CreateEntity(ctx, wqo, map[string][]string{"Content-Type": {"application/ld+json"}})
					if err != nil {
						log.Info().Msgf("could not create entity for %s because %s", nutsCode, err.Error())
						continue
					}
				}
			}
		}
	}

	return nil
}

func newWaterQualityObserved(observationID string, latitude float64, longitude float64, observedAt string, decorators ...entities.EntityDecoratorFunc) (types.Entity, error) {
	if len(decorators) == 0 {
		return nil, fmt.Errorf("at least one property must be set in a WaterQualityObserved entity")
	}

	if !strings.HasPrefix(observationID, fiware.WaterQualityObservedIDPrefix) {
		observationID = fiware.WaterQualityObservedIDPrefix + observationID
	}

	observationID = observationID + ":" + strings.ReplaceAll(observedAt, " ", "")

	decorators = append(decorators, DateObserved(observedAt), Location(latitude, longitude))

	e, err := entities.New(
		observationID, fiware.WaterQualityObservedTypeName,
		decorators...,
	)

	return e, err
}
