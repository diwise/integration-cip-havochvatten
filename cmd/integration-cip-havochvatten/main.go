package main

import (
	"context"
	"flag"
	"strings"

	"github.com/diwise/context-broker/pkg/ngsild/client"
	cip "github.com/diwise/integration-cip-havochvatten/internal/application/cip"
	"github.com/diwise/integration-cip-havochvatten/internal/application/havochvatten"
	"github.com/diwise/integration-cip-havochvatten/internal/application/lwm2m"
	"github.com/diwise/integration-cip-havochvatten/internal/application/models"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
)

const serviceName string = "integration-cip-havochvatten"

func main() {
	serviceVersion := buildinfo.SourceVersion()

	ctx, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	var nutsCodes string
	var endpoint string

	flag.StringVar(&nutsCodes, "nutscodes", "", "-nutscodes=SE00000,SE00001,SE00002")
	flag.StringVar(&endpoint, "endpoint", "", "-endpoint=<iotagent or cip>")
	flag.Parse()

	if nutsCodes == "" {
		logger.Fatal().Msg("at least one nutscode must be specified with -nutscodes")
	}

	if endpoint != "iot" && endpoint != "cip" {
		logger.Fatal().Msg("select one endpont -endpoint=<iot or cip>")
	}

	hovUrl := env.GetVariableOrDefault(logger, "HOV_BADPLATSEN_URL", "https://badplatsen.havochvatten.se/badplatsen/api")

	hovClient := havochvatten.New(hovUrl)
	
	temperatures, _ := hovClient.Load(ctx, func() []models.NutsCode {
		var codes []models.NutsCode
		nc := strings.Split(nutsCodes, ",")

		for _, n := range nc {
			codes = append(codes, models.NutsCode(n))
		}

		return codes
	}())

	if endpoint == "iot" {
		iotUrl := env.GetVariableOrDie(logger, "IOT_AGENT", "iot-agent URL")

		err := lwm2m.CreateTemperatures(ctx, temperatures, iotUrl)
		if err != nil {
			logger.Error().Err(err).Msg("unable to send smart data model")
		}		
	}

	if endpoint == "cip" {
		cipUrl := env.GetVariableOrDie(logger, "CONTEXT_BROKER_URL", "context broker URL")
		cbClient := client.NewContextBrokerClient(cipUrl)
		
		err := cip.CreateWaterQualityObserved(ctx, temperatures, cbClient)
		if err != nil {
			logger.Error().Err(err).Msg("unable to send smart data model")
		}
	}
}
