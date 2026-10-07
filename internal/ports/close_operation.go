// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"errors"
)

type (
	// CloseOperation adapts resource cleanup to the shared Close contract.
	CloseOperation func() error
)

// Close releases a resource through its cleanup callback.
func (closeOperation CloseOperation) Close() error { return errors.Join(closeOperation()) }
