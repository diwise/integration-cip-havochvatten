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
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
)

const (
	serviceName      string = "integration-cip-havochvatten"
	OutputTypeLwm2m  string = "lwm2m"
	OutputTypeFiware string = "fiware"
)

func main() {
	serviceVersion := buildinfo.SourceVersion()

	ctx, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	var nutsCodes string
	var outputType string
	var inputFilePath string

	flag.StringVar(&nutsCodes, "nutscodes", "", "-nutscodes=SE00000[=id1],SE00001[=id2],SE00002[=id3]")
	flag.StringVar(&outputType, "output", "", "-output=<lwm2m or fiware>")
	flag.StringVar(&inputFilePath, "input", "", "-input=<filename>")
	flag.Parse()

	if nutsCodes == "" && inputFilePath == "" {
		logger.Fatal().Msg("at least one nutscode must be specified with -nutscodes or a file with nutscodes with -input")
	}

	lwm2mUrl := env.GetVariableOrDefault(logger, "LWM2M_ENDPOINT_URL", "")
	cipUrl := env.GetVariableOrDefault(logger, "CONTEXT_BROKER_URL", "")
	hovUrl := env.GetVariableOrDefault(logger, "HOV_BADPLATSEN_URL", "https://badplatsen.havochvatten.se/badplatsen/api")

	if outputType == OutputTypeLwm2m {
		if lwm2mUrl == "" {
			logger.Fatal().Msg("no URL to lwm2m endpoint specified using env. var LWM2M_ENDPOINT_URL")
		}
	}

	if outputType == OutputTypeFiware {
		if cipUrl == "" {
			logger.Fatal().Msg("no URL to context broker specified using env. var CONTEXT_BROKER_URL")
		}
	}

	if outputType == "" {
		if lwm2mUrl != "" {
			outputType = OutputTypeLwm2m
		} else if cipUrl != "" {
			outputType = OutputTypeFiware
		}
	}

	if outputType == "" {
		logger.Fatal().Msg("no output type selected")
	}

	hovClient := havochvatten.New(hovUrl)

	nutsMap := make(map[string]string, 100)

	var codes []string
	if nutsCodes != "" {
		codes = strings.Split(nutsCodes, ",")
	}

	if inputFilePath != "" {
		in, err := os.Open(inputFilePath)
		if err != nil {
			panic(err)
		}
		defer in.Close()

		scan := bufio.NewScanner(in)
		for scan.Scan() {
			codes = append(codes, scan.Text())
		}
	}

	for _, code := range codes {
		pair := strings.Split(code, "=")
		if len(pair) == 2 {
			nutsMap[pair[0]] = pair[1]
		} else if len(pair) == 1 {
			nutsMap[pair[0]] = pair[0]
		} else {
			logger.Fatal().Msgf("invalid code: %s", code)
		}
	}

	temperatures, err := hovClient.Load(ctx, nutsMap)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load temperature data")
	}

	if outputType == OutputTypeLwm2m {
		err := lwm2m.CreateTemperatures(ctx, temperatures, lwm2mUrl)
		if err != nil {
			logger.Error().Err(err).Msg("unable to create lwm2m object")
		}
	}

	if outputType == OutputTypeFiware {
		cbClient := client.NewContextBrokerClient(cipUrl)

		err := cip.CreateWaterQualityObserved(ctx, temperatures, cbClient)
		if err != nil {
			logger.Error().Err(err).Msg("unable to send smart data model")
		}
	}
}
