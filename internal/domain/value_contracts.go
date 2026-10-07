// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// Normalizer explicitly scales supported absolute logical axes.
	Normalizer interface {
		Normalize(value float64) (float64, bool)
	}
	// UsageProvider exposes the standard HID page and usage identifiers.
	UsageProvider interface {
		Page() uint16
		ID() uint16
	}
	// OperationFailure supplies typed operation context independently of a wrapper.
	OperationFailure[ID any] interface {
		ErrorReporter
		Operation() string
		Device() ID
	}
	// LogicalBounds supplies inclusive absolute-axis limits.
	LogicalBounds interface{ Bounds() (int64, int64) }
)
