// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package implementation

import (
	"context"
	"errors"
	"fmt"

	"github.com/gostafa/goinput/internal/adapters/backoff"
	"github.com/gostafa/goinput/internal/adapters/singleton"
	"github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/platform"
	"github.com/gostafa/goinput/internal/ports"
)

const (
	defaultBufferSize = 0
)

// NewProvider owns a lazy coordinator shared by explicitly related managers.
func NewProvider() (ports.Provider[*application.Coordinator], error) {
	factory, err := platform.Factory()
	if err != nil {
		return nil, fmt.Errorf("create input system: %w", err)
	}

	provider := singleton.New(func(context.Context) (*application.Coordinator, error) {
		return application.NewCoordinator(factory, backoff.New()), nil
	})

	return provider, nil
}

// New creates the input manager using the registered platform backend.
func New(
	options domain.Options,
	coordinator ports.Provider[*application.Coordinator],
) (*application.Manager, error) {
	if options.BufferSize < defaultBufferSize {
		return nil, domain.ErrInvalidOptions
	}

	result0, callErr := application.NewManager(options, coordinator)
	if callErr != nil {
		return nil, errors.Join(callErr)
	}

	return result0, nil
}
