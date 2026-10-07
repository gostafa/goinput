// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Retrier retries only errors explicitly classified as transient, within a shared
	// discovery budget. Implementations must check context before the first attempt.
	Retrier interface {
		domain.Retrier
	}
)
