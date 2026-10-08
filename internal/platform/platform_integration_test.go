// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package platform_test

import (
	"errors"
	"testing"

	"github.com/gostafa/goinput/internal/domain"
	subject "github.com/gostafa/goinput/internal/platform"
	"github.com/gostafa/goinput/internal/ports"
)

func TestFactoryMatchesPlatformSupport(t *testing.T) {
	t.Parallel()

	factory, err := subject.Factory()
	if err != nil {
		checkUnsupportedFactory(t, factory, err)

		return
	}

	if factory == nil {
		t.Fatal("supported platform has no backend factory")
	}
}

func checkUnsupportedFactory(t *testing.T, factory ports.Factory, err error) {
	t.Helper()

	if factory != nil || !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("unsupported factory: %v, error=%v", factory, err)
	}
}
