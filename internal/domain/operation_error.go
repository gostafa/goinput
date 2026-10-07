// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// OpError is an operation error with a domain device identifier.
	OpError = OperationError[DeviceID]
)
