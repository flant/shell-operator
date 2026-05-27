// Package retry provides context-aware retry-with-backoff utilities.
package retry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Config controls the parameters for [RetryWithBackoff].
type Config struct {
	// MaxRetries is the maximum number of retry attempts (not counting the
	// initial call).  The total number of calls to fn is MaxRetries+1.
	MaxRetries int
	// InitialBackoff is the delay before the first retry; it doubles on each
	// consecutive failure up to MaxBackoff.
	InitialBackoff time.Duration
	// MaxBackoff caps the exponential backoff delay.
	MaxBackoff time.Duration
}

// RetryWithBackoff retries fn with exponential backoff, honouring ctx
// cancellation during sleep intervals.  The caller label is used in log
// messages.  Returns the last error from fn if all attempts fail, or nil on
// success.  If ctx is cancelled during a backoff sleep, ctx.Err() is returned
// immediately.
func WithBackoff(ctx context.Context, cfg Config, logger *log.Logger, caller string, fn func() error) error {
	backoff := cfg.InitialBackoff
	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt < cfg.MaxRetries {
			logger.Warn("retrying after failure",
				slog.String("caller", caller),
				slog.Int("attempt", attempt+1),
				slog.Duration("backoff", backoff),
				log.Err(lastErr))

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
			}
			backoff = min(backoff*2, cfg.MaxBackoff)
		}
	}
	return lastErr
}
