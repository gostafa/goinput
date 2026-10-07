package domain

import (
	"fmt"
	"math"
	"slices"
)

// HID constructs a usage from its standard page and ID.
func HID(page, id uint16) Usage { return Usage(uint32(page)<<16 | uint32(id)) }

func (u Usage) Page() uint16   { return uint16(uint32(u) >> 16) }
func (u Usage) ID() uint16     { return uint16(u) }
func (u Usage) String() string { return fmt.Sprintf("%04x:%04x", u.Page(), u.ID()) }

// Normalize explicitly scales a logical absolute axis into [0,1]. It does not
// infer a centered axis, deadzone, or physical unit. Invalid/null values, relative
// axes, hats, and unknown or degenerate ranges return false.
func (c Control) Normalize(value float64) (float64, bool) {
	if c.Kind != ControlAxis || c.Mode != AxisAbsolute || c.Range == nil ||
		c.Range.Max <= c.Range.Min || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	lo, hi := float64(c.Range.Min), float64(c.Range.Max)
	if hi <= lo || value < lo || value > hi {
		return 0, false
	}
	return (value - lo) / (hi - lo), true
}

// Hat decodes conventional four/eight-position HID hats with north at logical
// minimum. Unsupported encodings remain logical controls in their backend.
func Hat(value, min, max int64, hasNull bool) (HatDirection, bool) {
	if max < min || max-min > 7 || max-min < 3 {
		return HatNeutral, false
	}
	count := max - min + 1
	if count != 4 && count != 8 {
		return HatNeutral, false
	}
	if value < min || value > max {
		return HatNeutral, hasNull
	}
	return HatDirection((value - min) * (8 / count)), true
}

func CloneInfo(info DeviceInfo) DeviceInfo {
	info.Classes = slices.Clone(info.Classes)
	if info.VendorID != nil {
		n := *info.VendorID
		info.VendorID = &n
	}
	if info.ProductID != nil {
		n := *info.ProductID
		info.ProductID = &n
	}
	return info
}

func CloneCapabilities(caps Capabilities) Capabilities {
	caps.Controls = slices.Clone(caps.Controls)
	for i := range caps.Controls {
		if caps.Controls[i].Range != nil {
			r := *caps.Controls[i].Range
			caps.Controls[i].Range = &r
		}
	}
	return caps
}

func (e *OpError) Error() string {
	if e.DeviceID == "" {
		return fmt.Sprintf("goinput: %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("goinput: %s %s: %v", e.Op, e.DeviceID, e.Err)
}

func (e *OpError) Unwrap() error { return e.Err }
