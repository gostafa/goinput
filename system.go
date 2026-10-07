// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"errors"

	"github.com/gostafa/goinput/internal/implementation"
)

// NewSystem creates a reusable input system without opening native resources.
func NewSystem() (*System, error) {
	provider, err := implementation.NewProvider()
	if err != nil {
		return nil, errors.Join(publicError(err))
	}

	return &System{provider: provider}, nil
}
