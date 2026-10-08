// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"math"
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
		t.Run(
			examples[index].name,
			func(t *testing.T) { t.Parallel(); checkHat(t, &examples[index]) },
		)
	}
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
		fourNorthCase(),
		fourEastCase(),
		eightNorthwestCase(),
		nullHatCase(),
		outOfRangeHatCase(),
		unsupportedHatCase(),
		reversedHatCase(),
	}
}

func fourNorthCase() hatExample {
	return hatExample{
		"four north",
		Range{testVendorID, hatFourPositions},
		testVendorID,
		SupportUnsupported,
		HatNorth,
		true,
	}
}

func fourEastCase() hatExample {
	return hatExample{
		"four east",
		Range{testVendorID, hatFourPositions},
		testProductID,
		SupportUnsupported,
		HatEast,
		true,
	}
}

func eightNorthwestCase() hatExample {
	return hatExample{
		"eight northwest",
		Range{testFirstIndex, testHatMaximum},
		testHatMaximum,
		SupportUnsupported,
		HatNorthWest,
		true,
	}
}

func nullHatCase() hatExample {
	return hatExample{
		"null",
		Range{testFirstIndex, testHatMaximum},
		testHatNull,
		SupportSupported,
		HatNeutral,
		true,
	}
}

func outOfRangeHatCase() hatExample {
	return hatExample{
		"out of range",
		Range{testFirstIndex, testHatMaximum},
		testHatNull,
		SupportUnsupported,
		HatNeutral,
		false,
	}
}

func unsupportedHatCase() hatExample {
	return hatExample{
		"unsupported count",
		Range{testFirstIndex, hatFourPositions},
		testFirstIndex,
		SupportSupported,
		HatNeutral,
		false,
	}
}

func reversedHatCase() hatExample {
	return hatExample{
		"reversed",
		Range{testHatMaximum, testFirstIndex},
		testFirstIndex,
		SupportSupported,
		HatNeutral,
		false,
	}
}

type (
	normalizationExample struct {
		value            float64
		minimum, maximum int64
		want             float64
		valid            bool
	}
)

const (
	normalizationSpan     = 10
	normalizationMiddle   = 5
	normalizationFraction = 0.5
	testOperation         = "read"
	testEndpoint          = "endpoint"
)

func TestNormalizeLogicalBounds(t *testing.T) {
	t.Parallel()

	examples := normalizationExamples()
	for index := range examples {
		checkNormalization(t, &examples[index])
	}
}

func normalizationExamples() []normalizationExample {
	return []normalizationExample{
		{normalizationMiddle, testFirstIndex, normalizationSpan, normalizationFraction, true},
		{-testVendorID, testFirstIndex, normalizationSpan, normalizedMinimum, false},
		{
			normalizationSpan + testVendorID,
			testFirstIndex,
			normalizationSpan,
			normalizedMinimum,
			false,
		},
		{testVendorID, testFirstIndex, testFirstIndex, normalizedMinimum, false},
		{math.NaN(), testFirstIndex, normalizationSpan, normalizedMinimum, false},
		{math.Inf(testVendorID), testFirstIndex, normalizationSpan, normalizedMinimum, false},
	}
}

func checkNormalization(t *testing.T, example *normalizationExample) {
	t.Helper()

	control := absoluteTestControl()

	control.Range.Min, control.Range.Max = example.minimum, example.maximum

	got, valid := control.Normalize(example.value)

	if got != example.want || valid != example.valid {
		t.Fatalf("Normalize = (%v, %v), want (%v, %v)", got, valid, example.want, example.valid)
	}
}

func absoluteTestControl() *Control {
	control := new(Control)

	control.Kind, control.Mode = ControlAxis, AxisAbsolute
	control.Range = &Range{Min: testFirstIndex, Max: normalizationSpan}

	return control
}

func TestNormalizeRejectsMissingBoundsAndOtherControls(t *testing.T) {
	t.Parallel()

	control := absoluteTestControl()

	control.Range = nil
	checkRejectedNormalization(t, control)

	control.Mode = AxisRelative
	checkRejectedNormalization(t, control)

	control.Kind = ControlButton
	checkRejectedNormalization(t, control)
}

func checkRejectedNormalization(t *testing.T, control *Control) {
	t.Helper()

	value, valid := control.Normalize(normalizationMiddle)
	if value != normalizedMinimum || valid {
		t.Fatalf("unsupported normalization = (%v, %v)", value, valid)
	}
}
