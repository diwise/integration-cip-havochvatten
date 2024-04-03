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
		logger.Error("at least one nutscode must be specified with -nutscodes or a file with nutscodes with -input")
		os.Exit(1)
	}

	lwm2mUrl := env.GetVariableOrDefault(ctx, "LWM2M_ENDPOINT_URL", "http://iot-agent:8080/api/v0/messages/lwm2m")
	cipUrl := env.GetVariableOrDefault(ctx, "CONTEXT_BROKER_URL", "http://context-broker:8080")
	hovUrl := env.GetVariableOrDefault(ctx, "HOV_BADPLATSEN_URL", "https://badplatsen.havochvatten.se/badplatsen/api")

	if outputType == OutputTypeLwm2m {
		if lwm2mUrl == "" {
			logger.Error("no URL to lwm2m endpoint specified using env. var LWM2M_ENDPOINT_URL")
			os.Exit(1)
		}
	}

	if outputType == OutputTypeFiware {
		if cipUrl == "" {
			logger.Error("no URL to context broker specified using env. var CONTEXT_BROKER_URL")
			os.Exit(1)
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
		logger.Error("no output type selected")
		os.Exit(1)
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
			logger.Error("invalid code", "code", code)
			os.Exit(1)
		}
	}

	temperatures, err := hovClient.Load(ctx, nutsMap)
	if err != nil {
		logger.Error("failed to load temperature data", "err", err.Error())
		os.Exit(1)
	}

	if outputType == OutputTypeLwm2m {
		err := lwm2m.CreateTemperatures(ctx, temperatures, lwm2mUrl)
		if err != nil {
			logger.Error("unable to create lwm2m object", "err", err.Error())
			os.Exit(1)
		}
	}

	if outputType == OutputTypeFiware {
		cbClient := client.NewContextBrokerClient(cipUrl)

		err := cip.CreateWaterQualityObserved(ctx, temperatures, cbClient)
		if err != nil {
			logger.Error("unable to send smart data model", "err", err.Error())
			os.Exit(1)
		}
	}
}
