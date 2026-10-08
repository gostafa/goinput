// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package windows

import (
	"runtime"
	"testing"
)

func TestFactoryAvailability(t *testing.T) {
	t.Parallel()

	supported := runtime.GOOS == "windows" &&
		(runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64")
	if (Factory() != nil) != supported {
		t.Fatal("factory availability does not match the supported target")
	}
}
