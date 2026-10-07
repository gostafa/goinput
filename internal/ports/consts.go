// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"time"
)

const (
	// DiscoveryBudget bounds the entire discovery phase, including metadata retries.
	// Capture startup and active stream lifetimes use their caller/owner contexts.
	DiscoveryBudget = 500 * time.Millisecond
)
