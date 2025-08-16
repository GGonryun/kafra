package backoff

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

type Backoff struct {
	startDuration time.Duration
	maxDuration   time.Duration
	count         int
}

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

func (b *Backoff) Next() time.Duration {
	b.count++
	
	duration := time.Duration(float64(b.startDuration) * math.Pow(2, float64(b.count-1)))
	
	if duration > b.maxDuration {
		duration = b.maxDuration
	}
	
	// Add jitter: ±25% of the duration to prevent thundering herd
	jitterRange := float64(duration) * 0.25
	jitter := time.Duration(rand.Float64()*jitterRange*2 - jitterRange)
	duration += jitter
	
	// Ensure we don't go below 0 or above maxDuration
	if duration < 0 {
		duration = b.startDuration
	}
	if duration > b.maxDuration {
		duration = b.maxDuration
	}
	
	return duration
}

func (b *Backoff) Reset() {
	b.count = 0
}

func (b *Backoff) Count() int {
	return b.count
}