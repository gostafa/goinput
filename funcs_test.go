// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"errors"
	"math"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
)

type (
	normalizationCase struct {
		value float64
		want  float64
		valid bool
	}
)

const (
	normalizedMidpoint = 0.5
	normalizedMaximum  = 1
	hatTestMaximum     = 7
	axisTestLimit      = 10
	axisOutsideLimit   = 11
)

func TestNormalizeRange(t *testing.T) {
	t.Parallel()

	control := normalizationControl(
		ControlAxis,
		AxisAbsolute,
		&Range{Min: -axisTestLimit, Max: axisTestLimit},
	)
	candidates := normalizationCases()

	for index := range candidates {
		checkNormalization(t, &control, &candidates[index])
	}
}

func TestNormalizeUnsupportedControls(t *testing.T) {
	t.Parallel()

	controls := unsupportedNormalizationControls()
	for index := range controls {
		var valid bool

		_, valid = controls[index].Normalize(normalizedMinimum)

		if valid {
			t.Fatalf("unsupported control normalized: %+v", controls[index])
		}
	}
}

func TestPublicErrorSentinels(t *testing.T) {
	t.Parallel()

	pairs := [][errorPairSize]error{
		{domain.ErrUnsupported, ErrUnsupported},
		{domain.ErrPermissionDenied, ErrPermissionDenied},
		{domain.ErrNotFound, ErrNotFound},
		{domain.ErrClosed, ErrClosed},
		{domain.ErrDisconnected, ErrDisconnected},
		{domain.ErrEventLoss, ErrEventLoss},
		{domain.ErrRegistrationConflict, ErrRegistrationConflict},
		{domain.ErrInvalidOptions, ErrInvalidOptions},
	}
	for index := range pairs {
		checkPublicSentinel(t, pairs[index][0], pairs[index][1])
	}
}

func TestPublicErrorPreservesUnknownCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("driver failed")
	original := &domain.OpError{Op: "open", DeviceID: "test-device", Err: cause}
	translated := publicError(original)

	var operation *OpError

	if !errors.As(translated, &operation) || !errors.Is(translated, cause) {
		t.Fatalf("operation and cause were not preserved: %v", translated)
	}

	if operation.Op != original.Op || operation.DeviceID != DeviceID(original.DeviceID) {
		t.Fatalf("operation metadata changed: %+v", operation)
	}
}

func normalizationControl(kind ControlKind, mode AxisMode, logical *Range) Control {
	return Control{
		ID: "", Name: "", Usage: UsageUnknown, Mapping: MappingUnknown,
		Unit: UnitUnknown, Support: SupportUnknown, Kind: kind, Mode: mode, Range: logical,
	}
}

func normalizationCases() []normalizationCase {
	return []normalizationCase{
		{-axisTestLimit, normalizedMinimum, true},
		{normalizedMinimum, normalizedMidpoint, true},
		{axisTestLimit, normalizedMaximum, true},
		{-axisOutsideLimit, normalizedMinimum, false},
		{axisOutsideLimit, normalizedMinimum, false},
		{math.NaN(), normalizedMinimum, false},
		{math.Inf(normalizedMaximum), normalizedMinimum, false},
	}
}

func unsupportedNormalizationControls() []Control {
	return []Control{
		normalizationControl(
			ControlAxis,
			AxisRelative,
			&Range{Min: -axisTestLimit, Max: axisTestLimit},
		),
		normalizationControl(
			ControlHat,
			AxisAbsolute,
			&Range{Min: normalizedMinimum, Max: hatTestMaximum},
		),
		normalizationControl(ControlAxis, AxisAbsolute, nil),
		normalizationControl(
			ControlAxis,
			AxisAbsolute,
			&Range{Min: normalizedMaximum, Max: normalizedMaximum},
		),
	}
}

func checkNormalization(t *testing.T, control *Control, example *normalizationCase) {
	t.Helper()

	got, valid := control.Normalize(example.value)
	if got != example.want || valid != example.valid {
		t.Fatalf(
			"Normalize(%v) = (%v, %v), want (%v, %v)",
			example.value,
			got,
			valid,
			example.want,
			example.valid,
		)
	}
}

func checkPublicSentinel(t *testing.T, original, expected error) {
	t.Helper()

	got := publicError(original)
	if !errors.Is(got, expected) {
		t.Fatalf("publicError(%v) = %v, want %v", original, got, expected)
	}
}
