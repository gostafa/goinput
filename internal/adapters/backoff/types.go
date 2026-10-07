// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoff

import (
	"context"

	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Retrier implements the application retry port with fixed discovery defaults.
	Retrier   interface{ domain.Retrier }
	retryFunc func(context.Context, func(context.Context) error, func(error) bool) error
)
