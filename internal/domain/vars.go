// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"errors"
)

var (
	// ErrUnsupported indicates that the platform or operation is unavailable.
	ErrUnsupported = errors.New("goinput: unsupported platform or feature")
	// ErrPermissionDenied indicates insufficient access to an input endpoint.
	ErrPermissionDenied = errors.New("goinput: permission denied")
	// ErrNotFound indicates that the requested endpoint is unavailable.
	ErrNotFound = errors.New("goinput: device not found")
	// ErrClosed indicates that the manager or stream has closed.
	ErrClosed = errors.New("goinput: closed")
	// ErrDisconnected indicates that the device disconnected.
	ErrDisconnected = errors.New("goinput: device disconnected")
	// ErrEventLoss indicates that event delivery lost data.
	ErrEventLoss = errors.New("goinput: input events lost; reopen the device")
	// ErrRegistrationConflict indicates conflicting native backend registrations.
	ErrRegistrationConflict = errors.New("goinput: raw input registration conflict")
	// ErrInvalidOptions indicates invalid manager configuration.
	ErrInvalidOptions = errors.New("goinput: invalid options")
)
