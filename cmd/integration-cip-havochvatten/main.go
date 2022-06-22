package main

import (
	"context"
	"flag"
	"strings"

	"github.com/diwise/context-broker/pkg/ngsild/client"
	"github.com/diwise/integration-cip-havochvatten/internal/application"
	"github.com/diwise/integration-cip-havochvatten/internal/application/havochvatten"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
)

func main() {
	serviceVersion := buildinfo.SourceVersion()
	serviceName := "integration-cip-havochvatten"

	ctx, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	var nutsCodes string
	flag.StringVar(&nutsCodes, "nutscodes", "", "-nutscodes=SE00000,SE00001,SE00002")
	flag.Parse()

	havOchVattenApiUrl := env.GetVariableOrDefault(logger, "HAVOCHVATTEN_API", "https://badplatsen.havochvatten.se/badplatsen/api")
	contextBrokerUrl := env.GetVariableOrDefault(logger, "CONTEXT_BROKER", "http://localhost:1026")

	hovClient := havochvatten.New(havOchVattenApiUrl)
	contextbroker := client.NewContextBrokerClient(contextBrokerUrl)

	app := application.New(hovClient, contextbroker)

	app.CreateWaterQualityObserved(ctx, func() []application.NutsCode {
		var codes []application.NutsCode
		nc := strings.Split(nutsCodes, ",")

		for _, n := range nc {
			codes = append(codes, application.NutsCode(n))
		}

		return codes
	})

}
