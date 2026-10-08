// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"errors"

	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
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
	ErrInvalidOptions                   = errors.New("goinput: invalid options")
	_                 Device            = (*device)(nil)
	_                 ExtensionProvider = (*device)(nil)
	_                 managerImpl       = (*managerOperations[
		domain.DeviceInfo, domain.DeviceID, application.Device,
	])(
		nil,
	)
)
