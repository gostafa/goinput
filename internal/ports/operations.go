// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"

	"github.com/gostafa/goinput/internal/domain"
)

type (
	// ManagerOperations adapts typed operations to a manager without exposing state.
	ManagerOperations[I, ID, D any] struct {
		CloseOperation

		// Operations holds the immutable manager dispatch table.
		Operations managerDispatch[I, ID, D]
	}
	managerDispatch[I, ID, D any] struct {
		Devices func(context.Context) ([]I, error)
		Open    func(context.Context, ID) (D, error)
	}
	// DeviceOperations adapts typed operations to an event consumer.
	DeviceOperations[I, C, E any] struct {
		CloseOperation

		// Operations holds the immutable device dispatch table.
		Operations deviceDispatch[I, C, E]
	}
	deviceDispatch[I, C, E any] struct {
		Info         func() I
		Capabilities func() C
		Read         func(context.Context) (E, error)
		Extension    func(any) bool
	}
	// CaptureOperations adapts typed operations to a native capture.
	CaptureOperations struct {
		CloseOperation

		// Operations holds the immutable capture dispatch table.
		Operations captureDispatch[domain.DeviceInfo, domain.Capabilities]
	}
	captureDispatch[I, C any] struct {
		Info         func() I
		Capabilities func() C
		Extension    func(any) bool
	}
	// BackendOperations adapts typed discovery and capture operations to a backend.
	BackendOperations struct {
		CloseOperation

		// Operations holds the immutable backend dispatch table.
		Operations backendDispatch[domain.DeviceInfo, domain.DeviceID, EventSink, Capture]
	}
	backendDispatch[I, ID, S, C any] struct {
		Discover func(context.Context) ([]I, error)
		Open     func(context.Context, ID, S) (C, error)
	}
	// SinkOperations adapts publication and failure callbacks to an event sink.
	SinkOperations[E any] struct {
		// Operations holds the immutable sink dispatch table.
		Operations sinkDispatch[E]
	}
	sinkDispatch[E any] struct {
		Publish func(*E) bool
		Fail    func(error)
	}
)
