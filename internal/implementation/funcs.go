// Package implementation wires the private input runtime.
package implementation

import (
	"context"
	"github.com/gostafa/goinput/internal/adapters/backoff"
	_ "github.com/gostafa/goinput/internal/adapters/evdev"
	_ "github.com/gostafa/goinput/internal/adapters/iokit"
	"github.com/gostafa/goinput/internal/adapters/singleton"
	_ "github.com/gostafa/goinput/internal/adapters/win32"
	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/platform"
)

var coordinator = singleton.New(newCoordinator)

func newCoordinator(context.Context) (*application.Coordinator, error) {
	factory, err := platform.Factory()
	if err != nil {
		return nil, err
	}
	return application.NewCoordinator(factory, backoff.Retrier{}), nil
}
func New(options domain.Options) (*application.Manager, error) {
	if options.BufferSize < 0 {
		return nil, domain.ErrInvalidOptions
	}
	if _, err := platform.Factory(); err != nil {
		return nil, err
	}
	return application.NewManager(options, coordinator)
}
