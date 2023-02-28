package main

import (
	"bufio"
	"context"
	"flag"
	"os"
	"strings"

	"github.com/diwise/context-broker/pkg/ngsild/client"
	"github.com/diwise/integration-cip-havochvatten/internal/application/cip"
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
	var output string
	var input string

	flag.StringVar(&nutsCodes, "nutscodes", "", "-nutscodes=SE00000,SE00001,SE00002")
	flag.StringVar(&output, "output", "", "-output=<lwm2m or fiware>")
	flag.StringVar(&input, "input", "", "-input=<filename>")
	flag.Parse()

	if nutsCodes == "" && input == "" {
		logger.Fatal().Msg("at least one nutscode must be specified with -nutscodes or a file with nutscodes with -input")
	}

	if output != "lwm2m" && output != "fiware" {
		logger.Fatal().Msg("select one endpoint -output=<lwm2m or fiware>")
	}

	hovUrl := env.GetVariableOrDefault(logger, "HOV_BADPLATSEN_URL", "https://badplatsen.havochvatten.se/badplatsen/api")

	hovClient := havochvatten.New(hovUrl)

	if input != "" {
		in, err := os.Open(input)
		if err != nil {
			panic(err)
		}
		defer in.Close()

		scan := bufio.NewScanner(in)

		if len(nutsCodes) > 0 {
			nutsCodes = nutsCodes + ","
		}

		for scan.Scan() {
			nutsCodes += scan.Text() + ","
		}
	}

	temperatures, _ := hovClient.Load(ctx, func() []models.NutsCode {
		var codes []models.NutsCode
		nc := strings.Split(nutsCodes, ",")

		for _, n := range nc {
			if n == "" {
				continue
			}

			codes = append(codes, models.NutsCode(n))
		}

		return codes
	}())

	if output == "lwm2m" {
		lwm2mUrl := env.GetVariableOrDie(logger, "LWM2M_ENDPOINT_URL", "lwm2m endpoint URL")

		err := lwm2m.CreateTemperatures(ctx, temperatures, lwm2mUrl)
		if err != nil {
			logger.Error().Err(err).Msg("unable to create lwm2m object")
		}
	}

	if output == "fiware" {
		cipUrl := env.GetVariableOrDie(logger, "CONTEXT_BROKER_URL", "context broker URL")
		cbClient := client.NewContextBrokerClient(cipUrl)

		err := cip.CreateWaterQualityObserved(ctx, temperatures, cbClient)
		if err != nil {
			logger.Error().Err(err).Msg("unable to send smart data model")
		}
	}
}
