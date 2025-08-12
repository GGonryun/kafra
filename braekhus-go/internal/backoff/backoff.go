package backoff

import (
	"fmt"
	"math"
	"time"
)

// Backoff implements exponential backoff with jitter
type Backoff struct {
	startDuration time.Duration
	maxDuration   time.Duration
	count         int
}

// New creates a new Backoff instance
func New(startDuration, maxDuration time.Duration) (*Backoff, error) {
	if startDuration <= 0 {
		return nil, fmt.Errorf("startDuration must be greater than 0")
	}
	if maxDuration < startDuration {
		return nil, fmt.Errorf("maxDuration must be greater than or equal to startDuration")
	}
	
	return &Backoff{
		startDuration: startDuration,
		maxDuration:   maxDuration,
		count:         0,
	}, nil
}

// Next returns the next backoff duration
func (b *Backoff) Next() time.Duration {
	b.count++
	
	// Calculate exponential backoff: startDuration * 2^(count-1)
	duration := time.Duration(float64(b.startDuration) * math.Pow(2, float64(b.count-1)))
	
	// Cap at maxDuration
	if duration > b.maxDuration {
		duration = b.maxDuration
	}
	
	return duration
}

// Reset resets the backoff counter
func (b *Backoff) Reset() {
	b.count = 0
}

// Count returns the current retry count
func (b *Backoff) Count() int {
	return b.count
}