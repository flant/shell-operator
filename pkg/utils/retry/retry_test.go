package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestRetryWithBackoff_Success(t *testing.T) {
	cfg := Config{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 2 {
			return errors.New("transient")
		}
		return nil
	}

	err := WithBackoff(context.Background(), cfg, log.NewNop(), "test", fn)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRetryWithBackoff_Exhausted(t *testing.T) {
	cfg := Config{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
	}
	fn := func() error { return errors.New("fail") }

	err := WithBackoff(context.Background(), cfg, log.NewNop(), "test", fn)
	assert.Error(t, err)
	assert.EqualError(t, err, "fail")
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	cfg := Config{
		MaxRetries:     5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
	}
	fn := func() error { return errors.New("fail") }

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := WithBackoff(ctx, cfg, log.NewNop(), "test", fn)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "expected DeadlineExceeded, got: %v", err)
}

func TestRetryWithBackoff_BackoffCapsAtMax(t *testing.T) {
	cfg := Config{
		MaxRetries:     5,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	}
	attempts := 0
	fn := func() error {
		attempts++
		if attempts == 4 {
			return nil
		}
		return errors.New("fail")
	}

	err := WithBackoff(context.Background(), cfg, log.NewNop(), "test", fn)
	assert.NoError(t, err)
	assert.Equal(t, 4, attempts)
}