// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton_test

import (
	"context"
	"testing"

	subject "github.com/gostafa/goinput/internal/adapters/singleton"
	"github.com/gostafa/goinput/internal/ports"
)

const (
	noFactoryCalls    = 0
	testProviderValue = 42
)

func TestProviderInitializesLazilyAndSharesValue(t *testing.T) {
	t.Parallel()

	calls := new(int)
	provider := subject.New(func(context.Context) (int, error) {
		*calls++

		return testProviderValue, nil
	})

	if *calls != noFactoryCalls {
		t.Fatal("provider initialized eagerly")
	}

	checkProviderValue(t, provider)
	checkProviderValue(t, provider)
}

func checkProviderValue(t *testing.T, provider ports.Provider[int]) {
	t.Helper()

	value, err := provider.Get(t.Context())
	if value != testProviderValue || err != nil {
		t.Fatalf("provider value = (%v, %v)", value, err)
	}
}
