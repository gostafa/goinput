// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

import (
	"slices"
)

// NativeControls returns an independent copy of the descriptor mappings.
func (info InfoRecord[C]) NativeControls() []C { return slices.Clone(info.Controls) }

// Clone copies endpoint metadata and its control slice. Caller-defined control
// records are copied by value; nested references within those records are shared.
func (info InfoRecord[C]) Clone() InfoRecord[C] {
	return InfoRecord[C]{
		Controls: slices.Clone(info.Controls), RawInputHandle: info.RawInputHandle,
		DeviceType: info.DeviceType, Version: info.Version,
		UsagePage: info.UsagePage, Usage: info.Usage,
	}
}

// Clone returns an independent native control descriptor.
func (control NativeControl) Clone() NativeControl {
	return NativeControl{
		DescriptorBounds: control.DescriptorBounds,
		ID:               control.ID, Units: control.Units, UnitsExponent: control.UnitsExponent,
		VirtualKey: control.VirtualKey, BitSize: control.BitSize,
		ReportCount: control.ReportCount, LinkCollection: control.LinkCollection,
		DataIndex: control.DataIndex, ScanCode: control.ScanCode, ReportID: control.ReportID,
		HasNull: control.HasNull, Absolute: control.Absolute,
	}
}
