// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// DeviceID identifies a current operating-system input endpoint.
	// IDs may be reused after disconnection and are not persistent hardware IDs.
	DeviceID = identifier[interface{ deviceID() }]

	// ControlID identifies a control within a device, independently of its HID usage.
	ControlID = identifier[interface{ controlID() }]

	// Usage packs a HID usage page in its upper 16 bits and a usage ID in its lower 16.
	// Zero means no known usage. Vendor-defined usages are preserved.
	Usage uint32

	// identifier retains distinct device and control identities in generic records.
	identifier[Tag any] string
)
