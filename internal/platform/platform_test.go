// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package platform

import (
	"runtime"
	"testing"
)

func TestNativeFactorySelection(t *testing.T) {
	t.Parallel()

	targets := [...]string{"darwin", "linux", "windows", "unsupported"}
	for index := range targets {
		target := targets[index]
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			checkTargetFactory(t, target)
		})
	}
}

func checkTargetFactory(t *testing.T, target string) {
	t.Helper()

	if (nativeFactory(target) != nil) != supportsTarget(target) {
		t.Fatal("factory availability does not match the selected target")
	}
}

func supportsTarget(target string) bool {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return target == runtime.GOOS
	default:
		return false
	}
}
