// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"github.com/gostafa/goinput/internal/domain"
)

type (
	// Timestamp distinguishes source event time from host receipt time.
	// Device timestamps need not be monotonic or comparable between devices.
	Timestamp = domain.TimestampRecord[TimestampSource]

	// Event is a control update: digital values are 0/1, axes carry positions or deltas,
	// and hats use HatDirection. Values retain fractional scrolling.
	Event = domain.EventRecord[Timestamp, DeviceID, ControlID, EventAction]

	// OpError preserves a cause with operation and device context.
	OpError = domain.OperationError[DeviceID]
)
