// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

type (
	// DeviceID identifies a current operating-system input endpoint.
	// IDs may be reused after disconnection and are not persistent hardware IDs.
	DeviceID string

	// ControlID identifies a control within a device, independently of its HID usage.
	ControlID string

	// Usage packs a HID usage page in its upper 16 bits and a usage ID in its lower 16.
	// Zero means no known usage. Vendor-defined usages are preserved.
	Usage uint32
)
