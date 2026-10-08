// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package capture_test

import (
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	subject "github.com/gostafa/goinput/internal/ports/capture"
)

const (
	testSnapshot   = 1
	testIdentifier = "endpoint"
)

func TestCaptureDispatchUsesConfiguredSnapshots(t *testing.T) {
	t.Parallel()

	capture := new(subject.Operations[int, int])

	capture.Operations.Info, capture.Operations.Capabilities = snapshotDispatch, snapshotDispatch
	capture.Operations.Close, capture.Operations.Extension = closedDispatch, extensionDispatch
	checkDiagnostic(t, capture.Close())

	condition1 := capture.Info() != testSnapshot || capture.Capabilities() != testSnapshot ||
		!capture.Extension(capture)

	if condition1 {
		t.Fatal("capture dispatch lost snapshots or extension target")
	}
}

func closedDispatch() error { return domain.ErrClosed }

func snapshotDispatch() int { return testSnapshot }

func extensionDispatch(target any) bool { return target != nil }

func checkDiagnostic(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, domain.ErrClosed) {
		t.Fatalf("dispatch diagnostic = %v", err)
	}
}
