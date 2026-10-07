// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

// Copy copies the control schema, including each control's mutable fields.
func (caps *CapabilitiesRecord[C, Supported]) Copy() CapabilitiesRecord[C, Supported] {
	return CapabilitiesRecord[C, Supported]{
		Controls: cloneValues(caps.Controls), Complete: caps.Complete, Repeat: caps.Repeat,
	}
}
