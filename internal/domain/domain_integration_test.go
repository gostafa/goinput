// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain_test

import (
	"errors"
	"math"
	"testing"

	subject "github.com/gostafa/goinput/internal/domain"
)

type (
	hatExample struct {
		name     string
		logical  subject.Range
		value    int64
		null     subject.Support
		want     subject.HatDirection
		accepted bool
	}
)

const (
	testFirstIndex           = 0
	testVendorID             = 1
	testProductID            = 2
	testReplacementVendorID  = 3
	testReplacementProductID = 4
	testHatMaximum           = 7
	testHatNull              = 8
)

func TestHatCompassDirections(t *testing.T) {
	t.Parallel()
	checkCompassDirections(
		t,
		[]subject.HatDirection{
			subject.HatNorth,
			subject.HatNorthEast,
			subject.HatEast,
			subject.HatSouthEast,
			subject.HatSouth,
			subject.HatSouthWest,
			subject.HatWest,
			subject.HatNorthWest,
		},
		testFirstIndex,
	)
	checkCompassDirections(
		t,
		[]subject.HatDirection{
			subject.HatNorth,
			subject.HatEast,
			subject.HatSouth,
			subject.HatWest,
		},
		testVendorID,
	)
}

func TestCloneInfoIndependence(t *testing.T) {
	t.Parallel()

	vendor, product := uint16(testVendorID), uint16(testProductID)
	want := [testProductID]uint16{vendor, product}
	original := subject.DeviceInfo{
		ID:           "",
		Name:         "",
		Path:         "",
		Manufacturer: "",
		Serial:       "",
		Transport:    subject.TransportUnknown,
		VendorID:     &vendor,
		ProductID:    &product,
		Classes:      []subject.DeviceClass{subject.ClassMouse},
	}
	clone := subject.CloneInfo(&original)

	*clone.VendorID = testReplacementVendorID
	*clone.ProductID = testReplacementProductID
	clone.Classes[testFirstIndex] = subject.ClassKeyboard

	checkOriginalInfo(t, &original, &want)
}

func checkHat(t *testing.T, example *hatExample) {
	t.Helper()

	got, accepted := subject.Hat(
		example.value,
		example.logical,
		example.null == subject.SupportSupported,
	)
	if got != example.want || accepted != example.accepted {
		t.Fatalf("Hat = (%v, %v), want (%v, %v)", got, accepted, example.want, example.accepted)
	}
}

func checkCompassDirections(t *testing.T, directions []subject.HatDirection, minimum int64) {
	t.Helper()

	for ordinal := range directions {
		checkHat(t, &hatExample{
			name:    "compass",
			logical: subject.Range{Min: minimum, Max: minimum + int64(len(directions)) - 1},
			value: minimum + int64(
				ordinal,
			),
			null:     subject.SupportUnsupported,
			want:     directions[ordinal],
			accepted: true,
		})
	}
}

func checkOriginalInfo(t *testing.T, original *subject.DeviceInfo, want *[testProductID]uint16) {
	t.Helper()

	condition1 := *original.VendorID != want[testFirstIndex] ||
		*original.ProductID != want[testVendorID] ||
		original.Classes[testFirstIndex] != subject.ClassMouse

	if condition1 {
		t.Fatal("mutating cloned metadata changed the original")
	}
}

const (
	normalizationSpan     = 10
	normalizationMiddle   = 5
	normalizationFraction = 0.5
	testOperation         = "read"
	testEndpoint          = "endpoint"
)

func TestUsageRoundTrip(t *testing.T) {
	t.Parallel()

	usage := subject.HID(math.MaxUint16, math.MaxUint16)
	condition1 := usage.Page() != math.MaxUint16 || usage.ID() != math.MaxUint16 ||
		usage.String() != "ffff:ffff"

	if condition1 {
		t.Fatalf("usage round trip: %v", usage)
	}
}

func absoluteTestControl() *subject.Control {
	control := new(subject.Control)

	control.Kind, control.Mode = subject.ControlAxis, subject.AxisAbsolute
	control.Range = &subject.Range{Min: testFirstIndex, Max: normalizationSpan}

	return control
}

func TestCapabilitiesCloneOwnsLogicalBounds(t *testing.T) {
	t.Parallel()

	control := absoluteTestControl()
	caps := subject.Capabilities{
		Controls: []subject.Control{*control},
		Complete: true,
		Repeat:   subject.SupportUnsupported,
	}
	copyCaps := subject.CloneCapabilities(&caps)

	copyCaps.Controls[testFirstIndex].Range.Max = normalizationMiddle

	if caps.Controls[testFirstIndex].Range.Max != normalizationSpan || !copyCaps.Complete {
		t.Fatal("capability clone shares logical bounds or loses completeness")
	}
}

func TestCloneMissingOptionalMetadata(t *testing.T) {
	t.Parallel()

	info := new(subject.DeviceInfo)
	cloned := subject.CloneInfo(info)

	if cloned.VendorID != nil || cloned.ProductID != nil || cloned.Classes != nil {
		t.Fatal("absent metadata was populated while cloning")
	}
}

func TestOperationErrorMetadataAndCause(t *testing.T) {
	t.Parallel()

	operation := &subject.OpError{Op: testOperation, DeviceID: testEndpoint, Err: subject.ErrClosed}
	condition2 := operation.Operation() != testOperation || operation.Device() != testEndpoint ||
		!errors.Is(operation, subject.ErrClosed)

	if condition2 {
		t.Fatal("operation error lost its metadata or cause")
	}

	checkOperationMessage(t, operation)

	operation.DeviceID = ""
	checkOperationMessage(t, operation)
}

func checkOperationMessage(t *testing.T, operation *subject.OpError) {
	t.Helper()

	prefix := "goinput: " + testOperation

	if operation.DeviceID != "" {
		prefix += " " + string(operation.DeviceID)
	}

	if operation.Error() != prefix+": "+subject.ErrClosed.Error() {
		t.Fatalf("operation message = %q", operation.Error())
	}
}
