package retry

import (
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration.
type Config struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
}

// Default returns the default retry configuration.
func Default() Config {
	return Config{
		MaxAttempts: 3,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
	}
}

// Do executes fn with exponential backoff retry logic.
func Do(cfg Config, operation string, fn func() error) error {
	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt < cfg.MaxAttempts {
			wait := time.Duration(float64(cfg.InitialWait) * math.Pow(2, float64(attempt-1)))
			if wait > cfg.MaxWait {
				wait = cfg.MaxWait
			}
			fmt.Printf("  Retry %d/%d for %s (waiting %v): %v\n", attempt, cfg.MaxAttempts, operation, wait, lastErr)
			time.Sleep(wait)
		}
	}
	return fmt.Errorf("%s failed after %d attempts: %w", operation, cfg.MaxAttempts, lastErr)
}
