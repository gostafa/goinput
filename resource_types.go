// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"context"

	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	deviceview "github.com/gostafa/goinput/internal/ports/device"
	managerview "github.com/gostafa/goinput/internal/ports/manager"
)

type (
	// System owns the shared native session and its callback bindings. Construct with
	// NewSystem and reuse across managers; its zero value is not usable.
	System = systemProvider[ports.Provider[*application.Coordinator]]

	systemProvider[Provider any] struct {
		provider Provider
	}

	// Manager owns input captures. Construct with New; do not copy or use its zero value.
	Manager = managerOperations[DeviceInfo, DeviceID, Device]

	managerImpl interface {
		Devices(ctx context.Context) ([]domain.DeviceInfo, error)
		Open(ctx context.Context, id domain.DeviceID) (application.Device, error)
		Close() error
	}

	// Device is a concurrency-safe consumer whose readers share one ordered queue.
	Device interface {
		Info() DeviceInfo
		Capabilities() Capabilities
		Read(ctx context.Context) (Event, error)
		Close() error
	}

	// ExtensionProvider queries native metadata through a pointer to a supported struct.
	ExtensionProvider interface{ Extension(target any) bool }

	device = deviceOperations[DeviceInfo, Capabilities, Event]

	// translatedError retains a wrapper's text and custom matching while exposing
	// translated children to errors.Is and errors.As.
	translatedError struct {
		operations errorDispatch
	}

	errorDispatch struct {
		message func() string
		unwrap  func() []error
		match   func(error) bool
		assign  func(any) bool
	}

	deviceOperations[I, C, E any] = deviceview.Operations[I, C, E]

	managerOperations[I, ID, D any] = managerview.Operations[I, ID, D]
)
