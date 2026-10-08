// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32_test

import (
	"testing"

	subject "github.com/gostafa/goinput/extensions/win32"
)

const (
	firstControlIndex = 0
	testControlID     = "control"
	changedControlID  = "changed"
)

func TestNativeSnapshotsAreIndependent(t *testing.T) {
	t.Parallel()

	control := new(subject.NativeControl)

	control.ID = testControlID

	info := nativeTestInfo(control)
	cloned := info.Clone()

	cloned.Controls[firstControlIndex].ID = changedControlID

	checkNativeSnapshot(t, &info, control)
}

func nativeTestInfo(control *subject.NativeControl) subject.Info {
	return subject.Info{
		Controls: []subject.NativeControl{
			*control,
		},
		RawInputHandle: firstControlIndex,
		DeviceType:     firstControlIndex,
		Version:        firstControlIndex,
		UsagePage:      firstControlIndex,
		Usage:          firstControlIndex,
	}
}

func checkNativeSnapshot(t *testing.T, info *subject.Info, control *subject.NativeControl) {
	t.Helper()

	controls := info.NativeControls()

	controls[firstControlIndex].ID = changedControlID

	if info.Controls[firstControlIndex].ID != testControlID || control.Clone() != *control {
		t.Fatal("native descriptor snapshot shares a slice or loses fields")
	}
}
