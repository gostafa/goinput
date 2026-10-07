// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"testing"
)

type (
	hatExample struct {
		name     string
		logical  Range
		value    int64
		null     Support
		want     HatDirection
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

func TestHatEncodings(t *testing.T) {
	t.Parallel()

	examples := hatEncodingCases()
	for index := range examples {
		t.Run(examples[index].name, func(t *testing.T) { checkHat(t, &examples[index]) })
	}
}

func TestHatCompassDirections(t *testing.T) {
	t.Parallel()
	checkCompassDirections(
		t,
		[]HatDirection{
			HatNorth,
			HatNorthEast,
			HatEast,
			HatSouthEast,
			HatSouth,
			HatSouthWest,
			HatWest,
			HatNorthWest,
		},
		testFirstIndex,
	)
	checkCompassDirections(t, []HatDirection{HatNorth, HatEast, HatSouth, HatWest}, testVendorID)
}

func TestCloneInfoIndependence(t *testing.T) {
	t.Parallel()

	vendor, product := uint16(testVendorID), uint16(testProductID)
	wantVendor, wantProduct := vendor, product
	original := DeviceInfo{
		ID: "", Name: "", Path: "", Manufacturer: "", Serial: "", Transport: TransportUnknown,
		VendorID:  &vendor,
		ProductID: &product,
		Classes:   []DeviceClass{ClassMouse},
	}
	clone := CloneInfo(&original)

	*clone.VendorID = testReplacementVendorID
	*clone.ProductID = testReplacementProductID
	clone.Classes[testFirstIndex] = ClassKeyboard

	checkOriginalInfo(t, &original, wantVendor, wantProduct)
}

func checkHat(t *testing.T, example *hatExample) {
	t.Helper()

	got, accepted := Hat(example.value, example.logical, example.null == SupportSupported)
	if got != example.want || accepted != example.accepted {
		t.Fatalf("Hat = (%v, %v), want (%v, %v)", got, accepted, example.want, example.accepted)
	}
}

func hatEncodingCases() []hatExample {
	return []hatExample{
		{
			name:     "four north",
			logical:  Range{Min: testVendorID, Max: hatFourPositions},
			value:    testVendorID,
			null:     SupportUnsupported,
			want:     HatNorth,
			accepted: true,
		},
		{
			name:     "four east",
			logical:  Range{Min: testVendorID, Max: hatFourPositions},
			value:    testProductID,
			null:     SupportUnsupported,
			want:     HatEast,
			accepted: true,
		},
		{
			name:     "eight northwest",
			logical:  Range{Min: testFirstIndex, Max: testHatMaximum},
			value:    testHatMaximum,
			null:     SupportUnsupported,
			want:     HatNorthWest,
			accepted: true,
		},
		{
			name:     "null",
			logical:  Range{Min: testFirstIndex, Max: testHatMaximum},
			value:    testHatNull,
			null:     SupportSupported,
			want:     HatNeutral,
			accepted: true,
		},
		{
			name:     "out of range",
			logical:  Range{Min: testFirstIndex, Max: testHatMaximum},
			value:    testHatNull,
			null:     SupportUnsupported,
			want:     HatNeutral,
			accepted: false,
		},
		{
			name:     "unsupported count",
			logical:  Range{Min: testFirstIndex, Max: hatFourPositions},
			value:    testFirstIndex,
			null:     SupportSupported,
			want:     HatNeutral,
			accepted: false,
		},
		{
			name:     "reversed",
			logical:  Range{Min: testHatMaximum, Max: testFirstIndex},
			value:    testFirstIndex,
			null:     SupportSupported,
			want:     HatNeutral,
			accepted: false,
		},
	}
}

func checkCompassDirections(t *testing.T, directions []HatDirection, minimum int64) {
	t.Helper()

	for ordinal := range directions {
		checkHat(t, &hatExample{
			name:    "compass",
			logical: Range{Min: minimum, Max: minimum + int64(len(directions)) - 1},
			value: minimum + int64(
				ordinal,
			),
			null:     SupportUnsupported,
			want:     directions[ordinal],
			accepted: true,
		})
	}
}

func checkOriginalInfo(t *testing.T, original *DeviceInfo, wantVendor, wantProduct uint16) {
	t.Helper()

	if *original.VendorID != wantVendor || *original.ProductID != wantProduct ||
		original.Classes[testFirstIndex] != ClassMouse {
		t.Fatal("mutating cloned metadata changed the original")
	}
}
