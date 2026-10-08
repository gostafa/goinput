// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package implementation_test

import (
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	subject "github.com/gostafa/goinput/internal/implementation"
	"github.com/gostafa/goinput/internal/platform"
	"github.com/gostafa/goinput/internal/ports"
)

func TestProviderFactoryDiagnosticIsPreserved(t *testing.T) {
	t.Parallel()

	provider, err := subject.NewProvider(
		func() (ports.Factory, error) { return nil, domain.ErrUnsupported },
	)
	if provider != nil || !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("provider factory = (%v, %v)", provider, err)
	}
}

func TestProviderCreatesOnlyCoordinatorState(t *testing.T) {
	t.Parallel()

	provider, err := subject.NewProvider(platform.Factory)
	if err != nil {
		t.Fatal(err)
	}

	coordinator, err := provider.Get(t.Context())
	if err != nil || coordinator == nil {
		t.Fatalf("coordinator state = (%v, %v)", coordinator, err)
	}
}
