// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// Unit describes the interpretation of a control value.
	Unit = code[interface{ unit() }]
	// Support reports whether a capability is supported.
	Support = code[interface{ support() }]
	// EventAction describes a control transition.
	EventAction = code[interface{ eventAction() }]
	// TimestampSource identifies the origin of an event timestamp.
	TimestampSource = code[interface{ timestampSource() }]
	// HatDirection encodes a compass direction or neutral hat position.
	HatDirection int8
)
