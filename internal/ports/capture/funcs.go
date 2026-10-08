// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package capture

import (
	"errors"
)

// Info returns an endpoint snapshot.
func (view *Operations[I, C]) Info() I { return view.Operations.Info() }

// Capabilities returns a control-schema snapshot.
func (view *Operations[I, C]) Capabilities() C { return view.Operations.Capabilities() }

// Extension queries a typed native extension.
func (view *Operations[I, C]) Extension(
	target any,
) bool {
	return view.Operations.Extension(target)
}

// Close releases the view resources.
func (view *Operations[I, C]) Close() error { return errors.Join(view.Operations.Close()) }
