// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput_test

import (
	"context"
	"errors"
	"math"
	"testing"

	subject "github.com/gostafa/goinput"
)

type (
	normalizationCase struct {
		value float64
		want  float64
		valid bool
	}
)

const (
	testOpenOperation  = "open"
	normalizedMinimum  = 0
	errorPairSize      = 2
	normalizedMidpoint = 0.5
	normalizedMaximum  = 1
	hatTestMaximum     = 7
	axisTestLimit      = 10
	axisOutsideLimit   = 11

	invalidBufferSize = -1
)

func TestNormalizeRange(t *testing.T) {
	t.Parallel()

	control := normalizationControl(
		subject.ControlAxis,
		subject.AxisAbsolute,
		&subject.Range{Min: -axisTestLimit, Max: axisTestLimit},
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

func normalizationControl(
	kind subject.ControlKind,
	mode subject.AxisMode,
	logical *subject.Range,
) subject.Control {
	return subject.Control{
		ID:      "",
		Name:    "",
		Usage:   subject.UsageUnknown,
		Mapping: subject.MappingUnknown,
		Unit:    subject.UnitUnknown,
		Support: subject.SupportUnknown,
		Kind:    kind,
		Mode:    mode,
		Range:   logical,
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

func unsupportedNormalizationControls() []subject.Control {
	return []subject.Control{
		normalizationControl(
			subject.ControlAxis,
			subject.AxisRelative,
			&subject.Range{Min: -axisTestLimit, Max: axisTestLimit},
		),
		normalizationControl(
			subject.ControlHat,
			subject.AxisAbsolute,
			&subject.Range{Min: normalizedMinimum, Max: hatTestMaximum},
		),
		normalizationControl(subject.ControlAxis, subject.AxisAbsolute, nil),
		normalizationControl(
			subject.ControlAxis,
			subject.AxisAbsolute,
			&subject.Range{Min: normalizedMaximum, Max: normalizedMaximum},
		),
	}
}

func checkNormalization(t *testing.T, control *subject.Control, example *normalizationCase) {
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

func TestSystemCreatesLazyManager(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t)

	manager, err := subject.New(t.Context(), subject.Options{BufferSize: normalizedMinimum}, system)
	if err != nil {
		t.Fatal(err)
	}

	checkManagerClose(t, manager)
}

func TestNewRejectsNegativeBuffer(t *testing.T) {
	t.Parallel()

	manager, err := subject.New(
		t.Context(),
		subject.Options{BufferSize: invalidBufferSize},
		newTestSystem(t),
	)
	if manager != nil || !errors.Is(err, subject.ErrInvalidOptions) {
		t.Fatalf("invalid buffer = (%v, %v)", manager, err)
	}
}

func newTestSystem(t *testing.T) *subject.System {
	t.Helper()

	system, err := subject.NewSystem(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return system
}

func checkManagerClose(t *testing.T, manager *subject.Manager) {
	t.Helper()

	err := manager.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Close()
	if err != nil {
		t.Fatalf("repeated close = %v", err)
	}
}

func TestSystemCreationHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	system, err := subject.NewSystem(ctx)
	if system != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled system = (%v, %v)", system, err)
	}
}

func TestSystemCreationRejectsNilContext(t *testing.T) {
	t.Parallel()

	var ctx context.Context

	system, err := subject.NewSystem(ctx)

	if system != nil || !errors.Is(err, subject.ErrInvalidOptions) {
		t.Fatalf("nil system context = (%v, %v)", system, err)
	}
}

func TestNewRejectsUninitializedSystem(t *testing.T) {
	t.Parallel()

	systems := []*subject.System{nil, new(subject.System)}
	for index := range systems {
		manager, err := subject.New(t.Context(), subject.Options{BufferSize: 0}, systems[index])
		if manager != nil || !errors.Is(err, subject.ErrInvalidOptions) {
			t.Fatalf("New with uninitialized system = (%v, %v)", manager, err)
		}
	}
}

func TestPublicUsageRoundTrip(t *testing.T) {
	t.Parallel()

	usage := subject.HID(subject.PageGenericDesktop, normalizedMaximum)
	if usage.Page() != subject.PageGenericDesktop || usage.ID() != normalizedMaximum {
		t.Fatalf("usage round trip = %v", usage)
	}

	if usage.String() != "0001:0001" {
		t.Fatalf("usage string = %s", usage.String())
	}
}
