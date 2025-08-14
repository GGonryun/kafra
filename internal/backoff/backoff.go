package backoff

import (
	"fmt"
	"math"
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
	
	return duration
}

func (b *Backoff) Reset() {
	b.count = 0
}

func (b *Backoff) Count() int {
	return b.count
}