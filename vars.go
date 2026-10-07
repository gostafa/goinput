package goinput

import (
	"errors"

	"github.com/gostafa/goinput/internal/application"
)

var (
	ErrUnsupported          = errors.New("goinput: unsupported platform or feature")
	ErrPermissionDenied     = errors.New("goinput: permission denied")
	ErrNotFound             = errors.New("goinput: device not found")
	ErrClosed               = errors.New("goinput: closed")
	ErrDisconnected         = errors.New("goinput: device disconnected")
	ErrEventLoss            = errors.New("goinput: input events lost; reopen the device")
	ErrRegistrationConflict = errors.New("goinput: raw input registration conflict")
	ErrInvalidOptions       = errors.New("goinput: invalid options")
)

var _ Device = (*device)(nil)
var _ ExtensionProvider = (*device)(nil)
var _ managerImpl = (*application.Manager)(nil)
