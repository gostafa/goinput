// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

type (
	// DescriptorBounds groups logical and physical bounds of one report value.
	DescriptorBounds struct {
		// LogicalMin is the inclusive logical lower bound.
		LogicalMin int32
		// LogicalMax is the inclusive logical upper bound.
		LogicalMax int32
		// PhysicalMin is the descriptor's physical lower bound.
		PhysicalMin int32
		// PhysicalMax is the descriptor's physical upper bound.
		PhysicalMax int32
	}
)
