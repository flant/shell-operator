package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metrics"
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
)

const (
	AppName        = "shell-operator"
	AppDescription = "Shell-operator is a tool for running event-driven scripts in a Kubernetes cluster"
)

func start(logger *log.Logger, cfg *app.Config) func(cmd *cobra.Command, args []string) error {
	return func(_ *cobra.Command, _ []string) error {
		app.AppStartMessage = fmt.Sprintf("%s %s", app.AppName, app.Version)
		ctx := context.Background()
		telemetryShutdown := registerTelemetry(ctx)

		// Initialize metric names with the configured prefix.
		metrics.InitMetrics(cfg.App.PrometheusMetricsPrefix)

		// Init logging and initialize a ShellOperator instance.
		operator, err := shell_operator.Init(ctx, cfg, logger.Named("shell-operator"))
		if err != nil {
			return fmt.Errorf("init failed: %w", err)
		}

		operator.Start()

		// Listen for SIGUSR1 (soft reinit) and SIGUSR2 (full reset) in the background.
		go func() {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGUSR1, syscall.SIGUSR2)
			for sig := range ch {
				switch sig {
				case syscall.SIGUSR1:
					operator.Reinit(ctx, false)
				case syscall.SIGUSR2:
					operator.Reinit(ctx, true)
				}
			}
		}()

		// Block until OS signal.
		utils_signal.WaitForProcessInterruption(func() {
			operator.Shutdown()
			_ = telemetryShutdown(ctx)
			os.Exit(1)
		})

		return nil
	}
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
