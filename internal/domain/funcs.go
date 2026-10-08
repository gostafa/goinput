// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"fmt"
	"math"
	"math/bits"
	"slices"
)

// Copy copies the control schema, including each control's mutable fields.
func (caps *CapabilitiesRecord[C, Supported]) Copy() CapabilitiesRecord[C, Supported] {
	return CapabilitiesRecord[C, Supported]{
		Controls: cloneValues(caps.Controls), Complete: caps.Complete, Repeat: caps.Repeat,
	}
}

// Clone copies a control, including its optional mutable logical range.
func (control ControlRecord[R, ID, U, Kind, Mapping, Mode, ValueUnit, Supported]) Clone() ControlRecord[
	R, ID, U, Kind, Mapping, Mode, ValueUnit, Supported,
] {
	return ControlRecord[R, ID, U, Kind, Mapping, Mode, ValueUnit, Supported]{
		Range: clonePointer(
			control.Range,
		),
		ID:      control.ID,
		Name:    control.Name,
		Usage:   control.Usage,
		Kind:    control.Kind,
		Mapping: control.Mapping,
		Mode:    control.Mode,
		Unit:    control.Unit,
		Support: control.Support,
	}
}

// HID constructs a usage from its standard page and ID.
func HID(
	page, id uint16,
) Usage {
	return Usage(uint32(page)<<bits.Len16(math.MaxUint16) | uint32(id))
}

// Page returns the HID usage page.
func (u Usage) Page() uint16 {
	return uint16((uint32(u) >> bits.Len16(math.MaxUint16)) & math.MaxUint16)
}

// ID returns the usage ID within its HID page.
func (u Usage) ID() uint16 { return uint16(uint32(u) & math.MaxUint16) }

func (u Usage) String() string { return fmt.Sprintf("%04x:%04x", u.Page(), u.ID()) }

// Normalize explicitly scales a logical absolute axis into [0,1]. It does not
// infer a centered axis, deadzone, or physical unit. Invalid/null values, relative
// axes, hats, and unknown or degenerate ranges return false.
func (control *ControlRecord[R, ID, U, Kind, Mapping, Mode, ValueUnit, Supported]) Normalize(
	value float64,
) (float64, bool) {
	if control.Kind != Kind(ControlAxis) || control.Mode != Mode(AxisAbsolute) {
		return normalizedMinimum, false
	}

	return normalizeRecord(control.Range, value)
}

// Hat decodes conventional four/eight-position HID hats with north at logical
// minimum. Unsupported encodings remain logical controls in their backend.
func Hat(value int64, logical Range, hasNull bool) (HatDirection, bool) {
	count := logical.Max - logical.Min
	count++

	if !validHatRange(logical) || !validHatCount(count) {
		return HatNeutral, false
	}

	if value < logical.Min || value > logical.Max {
		return HatNeutral, hasNull
	}

	directions := [...]HatDirection{
		HatNorth, HatNorthEast, HatEast, HatSouthEast,
		HatSouth, HatSouthWest, HatWest, HatNorthWest,
	}

	return directions[(value-logical.Min)*(hatEightPositions/count)], true
}

// CloneInfo copies endpoint metadata and all mutable fields.
func CloneInfo(source *DeviceInfo) DeviceInfo {
	return source.Snapshot()
}

// CloneCapabilities copies controls and their logical ranges.
func CloneCapabilities(source *Capabilities) Capabilities {
	return source.Copy()
}

// Snapshot copies endpoint metadata and all mutable identifier and class fields.
func (info *DeviceInfoRecord[ID, Class, Medium]) Snapshot() DeviceInfoRecord[ID, Class, Medium] {
	return DeviceInfoRecord[ID, Class, Medium]{
		VendorID: clonePointer(info.VendorID), ProductID: clonePointer(info.ProductID),
		ID: info.ID, Name: info.Name, Path: info.Path, Manufacturer: info.Manufacturer,
		Serial: info.Serial, Classes: slices.Clone(info.Classes), Transport: info.Transport,
	}
}

func cloneValues[C Cloner[C]](source []C) []C {
	result := slices.Clone(source)
	for index := range result {
		result[index] = result[index].Clone()
	}

	return result
}

// Error formats operation and device context while retaining the cause's text.
func (e *OperationError[ID]) Error() string {
	return operationMessage(e.Op, string(e.DeviceID), e.Err)
}

// Unwrap returns the underlying cause for errors.Is and errors.As.
func (e *OperationError[ID]) Unwrap() error { return e.Err }

// Operation reports the failed operation.
func (e *OperationError[ID]) Operation() string { return e.Op }

// Device reports the affected endpoint, or an empty identifier.
func (e *OperationError[ID]) Device() ID { return e.DeviceID }

// Bounds returns the inclusive logical limits.
func (r Range) Bounds() (minimum, maximum int64) { return r.Min, r.Max }

func operationMessage(op, device string, err error) string {
	if device == "" {
		return fmt.Sprintf("goinput: %s: %v", op, err)
	}

	return fmt.Sprintf("goinput: %s %s: %v", op, device, err)
}

func normalizeRecord[R ~struct{ Min, Max int64 }](
	logical *R,
	value float64,
) (float64, bool) {
	if logical == nil || !finiteValue(value) {
		return normalizedMinimum, false
	}

	minimum, maximum := Range(*logical).Bounds()
	low, high := float64(minimum), float64(maximum)

	if !inFloatRange(value, low, high) {
		return normalizedMinimum, false
	}

	return (value - low) / (high - low), true
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}

	copyValue := *value

	return &copyValue
}

func finiteValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, int(normalizedMinimum))
}

func validHatRange(logical Range) bool {
	span := logical.Max - logical.Min

	return logical.Max >= logical.Min && span >= int64(UsageUnknown) && span < hatEightPositions
}

func validHatCount(count int64) bool {
	return count == hatFourPositions || count == hatEightPositions
}

func inFloatRange(value, low, high float64) bool {
	return high > low && value >= low && value <= high
}
