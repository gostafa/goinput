// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"errors"
	"testing"
)

func TestNewRejectsUninitializedSystem(t *testing.T) {
	t.Parallel()

	for _, system := range []*System{nil, new(System)} {
		manager, err := New(Options{BufferSize: 0}, system)
		if manager != nil || !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("New with uninitialized system = (%v, %v)", manager, err)
		}
	}
}
