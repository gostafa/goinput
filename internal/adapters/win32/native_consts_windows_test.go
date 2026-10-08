// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

const (
	testCapabilitiesText   = "capabilities"
	testAxisUsage          = 48
	testAxisValue          = 42
	testButtonPage         = 9
	testEleventhValue      = 11
	testOversizedPacket    = 25
	testTenthValue         = 10
	testUnknownValue       = 99
	testChangedText        = "changed"
	testNativePath         = "TEST-DEVICE"
	testNativeID           = "win32:test-device"
	testCaptureID          = "device"
	testMissingID          = "missing"
	testNativeCountFailure = "fixture exceeds native count"
)
