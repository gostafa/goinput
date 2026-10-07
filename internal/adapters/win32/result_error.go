// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

import (
	"errors"
)

// resultError retains the error when an operation's value is irrelevant.
func resultError[T any](_ T, err error) error { return errors.Join(err) }
