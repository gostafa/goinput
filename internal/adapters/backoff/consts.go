package backoff

import (
	"github.com/gostafa/goinput/internal/ports"
	"time"
)

const (
	budget          = ports.DiscoveryBudget
	initialInterval = 25 * time.Millisecond
	maximumInterval = 100 * time.Millisecond
	maxAttempts     = 4
	multiplier      = 2
	jitter          = 0.25
)
