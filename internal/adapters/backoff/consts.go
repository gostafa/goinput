// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoff

import (
	"time"

	"github.com/gostafa/goinput/internal/ports"
)

const (
	budget          = ports.DiscoveryBudget
	initialInterval = 25 * time.Millisecond
	maximumInterval = 100 * time.Millisecond
	maxAttempts     = 4
	multiplier      = 2
	jitter          = 0.25
)
