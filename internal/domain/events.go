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
		Timestamp Stamp
		DeviceID  Device
		ControlID Control
		Action    Action
		Value     float64
	}
	// OperationError adds operation and typed device context to an underlying cause.
	OperationError[ID ~string] struct {
		Err      error
		DeviceID ID
		Op       string
	}
	// Timestamp is the domain timestamp record.
	Timestamp = TimestampRecord[TimestampSource]
	// Event is the domain event record.
	Event = EventRecord[Timestamp, DeviceID, ControlID, EventAction]
)
