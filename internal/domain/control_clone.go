// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

// Clone copies a control, including its optional mutable logical range.
func (c ControlRecord[R, ID, U, Kind, Mapping, Mode, ValueUnit, Supported]) Clone() ControlRecord[
	R, ID, U, Kind, Mapping, Mode, ValueUnit, Supported,
] {
	return ControlRecord[R, ID, U, Kind, Mapping, Mode, ValueUnit, Supported]{
		Range: clonePointer(c.Range), ID: c.ID, Name: c.Name, Usage: c.Usage,
		Kind: c.Kind, Mapping: c.Mapping, Mode: c.Mode, Unit: c.Unit, Support: c.Support,
	}
}
