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
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
)

const (
	AppName        = "shell-operator"
	AppDescription = "Shell-operator is a tool for running event-driven scripts in a Kubernetes cluster"
)

// start returns the cobra RunE used by the "start" sub-command. It owns the
// process-wide concerns (signal handling, OpenTelemetry, log defaults) and
// delegates the actual lifecycle to shell-operator's Run method, which Starts,
// blocks until ctx is done, and synchronously Shuts down.
func start(logger *log.Logger, cfg *app.Config) func(cmd *cobra.Command, args []string) error {
	return func(_ *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		telemetryShutdown := registerTelemetry(ctx)
		defer func() { _ = telemetryShutdown(context.Background()) }()

		fmt.Println(app.AppName, app.Version)

		op, err := shell_operator.NewShellOperator(ctx, cfg, shell_operator.WithLogger(logger.Named("shell-operator")))
		if err != nil {
			return fmt.Errorf("build shell-operator: %w", err)
		}

		// Run blocks until ctx is canceled (typically by SIGINT/SIGTERM) and
		// performs a synchronous Shutdown before returning. No os.Exit on the
		// graceful path; the caller (cobra) maps a nil return to exit 0.
		return op.Run(ctx)
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
