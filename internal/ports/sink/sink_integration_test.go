// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package sink_test

import (
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	subject "github.com/gostafa/goinput/internal/ports/sink"
)

const (
	testSnapshot   = 1
	testIdentifier = "endpoint"
)

func TestSinkDispatchRetainsEventAndCause(t *testing.T) {
	t.Parallel()

	sink := new(subject.Operations[int])

	sink.Operations.Publish = func(value *int) bool { return *value == testSnapshot }
	sink.Operations.Fail = func(err error) { checkDiagnostic(t, err) }

	value := testSnapshot

	if !sink.Publish(&value) {
		t.Fatal("sink dispatch rejected the configured event")
	}

	sink.Fail(domain.ErrClosed)
}

func checkDiagnostic(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, domain.ErrClosed) {
		t.Fatalf("dispatch diagnostic = %v", err)
	}
}
