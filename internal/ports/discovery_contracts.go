// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Discoverer enumerates accessible endpoints.
	Discoverer[I any] interface{ domain.Discoverer[I] }
	// Opener subscribes an event sink to an endpoint.
	Opener[ID, S, C any] interface{ domain.Opener[ID, S, C] }
)
