// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application_test

import (
	"context"
	"errors"
	"testing"

	subject "github.com/gostafa/goinput/internal/application"
	"github.com/gostafa/goinput/internal/domain"
)

const (
	backendClosedMessage  = "backend closed %d times, want once"
	publishFailureMessage = "publish failed"
	testDeviceID          = "test-device"
	testQueueCapacity     = 2
	firstTestValue        = 1
	thirdTestValue        = 3
	fourthTestValue       = 4
	testEmptyBuffer       = 0
)

func TestManagerRejectsNilLifetime(t *testing.T) {
	t.Parallel()

	var ctx context.Context

	manager, err := subject.NewManager(ctx, domain.Options{BufferSize: testEmptyBuffer}, nil)
	if manager != nil || !errors.Is(err, domain.ErrInvalidOptions) {
		t.Fatalf("nil lifetime = (%v, %v)", manager, err)
	}
}
