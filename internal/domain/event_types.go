// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"time"
)

type (
	// TimestampRecord distinguishes source time from host receipt time.
	// Time is UTC; ReceivedAt retains its monotonic clock component.
	TimestampRecord[Source ~uint8] struct {
		// Time is the event time in UTC.
		Time time.Time
		// ReceivedAt is the host receipt time, including its monotonic component.
		ReceivedAt time.Time
		// Source identifies the origin of Time.
		Source Source
	}

	// EventRecord is one typed control update. Values preserve fractional scrolling.
	EventRecord[Stamp any, Device, Control ~string, Action ~uint8] struct {
		// Timestamp records the event and receipt clocks.
		Timestamp Stamp
		// DeviceID identifies the affected endpoint.
		DeviceID Device
		// ControlID identifies the changed device-local control.
		ControlID Control
		// Action describes the control transition.
		Action Action
		// Value preserves the reported scalar and fractional scrolling.
		Value float64
	}

	// OperationError adds operation and typed device context to an underlying cause.
	OperationError[ID ~string] struct {
		// Err retains the underlying cause.
		Err error
		// DeviceID identifies the affected endpoint.
		DeviceID ID
		// Op identifies the failed operation.
		Op string
	}

	// Timestamp is the domain timestamp record.
	Timestamp = TimestampRecord[TimestampSource]

	// Event is the domain event record.
	Event = EventRecord[Timestamp, DeviceID, ControlID, EventAction]
)
