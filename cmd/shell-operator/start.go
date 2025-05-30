package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/shell-operator/pkg/app"
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"gopkg.in/alecthomas/kingpin.v2"
)

func start(logger *log.Logger) func(_ *kingpin.ParseContext) error {

	app.AppStartMessage = fmt.Sprintf("%s %s", app.AppName, app.Version)
	ctx := context.Background()
	telemetryShutdown := registerTelemetry(ctx)
	// Init logging and initialize a ShellOperator instance.
	operator, err := shell_operator.Init(logger.Named("shell-operator"))
	if err != nil {
		os.Exit(1)
	}

	operator.Start()

	// Block action by waiting signals from OS.
	utils_signal.WaitForProcessInterruption(func() {
		operator.Shutdown()
		telemetryShutdown(ctx)
		os.Exit(1)
	})

	return nil
}

func registerTelemetry(ctx context.Context) func(ctx context.Context) error {
	endpoint := os.Getenv("TRACING_OTLP_ENDPOINT")
	authToken := os.Getenv("TRACING_OTLP_AUTH_TOKEN")

	if endpoint == "" {
		return func(_ context.Context) error {
			return nil
		}
	}

	opts := make([]otlptracegrpc.Option, 0, 1)

	opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
	opts = append(opts, otlptracegrpc.WithInsecure())

	if authToken != "" {
		opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{
			"Authorization": "Bearer " + strings.TrimSpace(authToken),
		}))
	}

	exporter, _ := otlptracegrpc.New(ctx, opts...)

	resource := sdkresource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(AppName),
		semconv.ServiceVersionKey.String(ShellOperatorVersion),
		semconv.TelemetrySDKLanguageKey.String("en"),
		semconv.K8SDeploymentName(AppName),
	)

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(provider)

	return provider.Shutdown
}
