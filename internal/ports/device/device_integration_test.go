// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package device_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	subject "github.com/gostafa/goinput/internal/ports/device"
)

const (
	testSnapshot   = 1
	testIdentifier = "endpoint"
)

func TestDeviceDispatchUsesConfiguredSnapshots(t *testing.T) {
	t.Parallel()

	device := new(subject.Operations[int, int, int])

	device.Operations.Info, device.Operations.Capabilities = snapshotDispatch, snapshotDispatch
	device.Operations.Close, device.Operations.Extension = closedDispatch, extensionDispatch
	device.Operations.Read = readDispatch

	value, err := device.Read(t.Context())
	checkDiagnostic(t, err)
	checkDeviceSnapshots(t, device, value)
}

func checkDeviceSnapshots(t *testing.T, device *subject.Operations[int, int, int], value int) {
	t.Helper()

	checkDiagnostic(t, device.Close())

	condition1 := device.Info() != testSnapshot || device.Capabilities() != testSnapshot ||
		!device.Extension(&value)

	if condition1 {
		t.Fatal("device dispatch lost snapshots or extension target")
	}
}

func closedDispatch() error { return domain.ErrClosed }

func snapshotDispatch() int { return testSnapshot }

func extensionDispatch(target any) bool { return target != nil }

func readDispatch(context.Context) (int, error) { return testSnapshot, domain.ErrClosed }

func checkDiagnostic(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, domain.ErrClosed) {
		t.Fatalf("dispatch diagnostic = %v", err)
	}
}
