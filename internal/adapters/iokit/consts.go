//go:build darwin && (amd64 || arm64)

package iokit

const (
	nativeOne                        = 1
	nativeFour                       = 4
	elementFeature                   = 257
	elementCollection                = 513
	nativeTwo                        = 2
	usageResolutionMultiplier        = 0x48
	nativeZero                       = 0
	ioNotPermitted            int32  = -536870174 // kIOReturnNotPermitted, 0xe00002e2.
	ioNotPrivileged           int32  = -536870207 // kIOReturnNotPrivileged, 0xe00002c1.
	ioNoDevice                int32  = -536870208 // kIOReturnNoDevice, 0xe00002c0.
	ioNotOpen                 int32  = -536870195 // kIOReturnNotOpen, 0xe00002cd.
	maxNativeElements                = 65536
	maxNativeString                  = 1 << 20
	utf8Encoding              uint32 = 0x08000100
	runLoopInterval                  = 0.02

	operationOpen        = "open"
	registryPathCapacity = 4096

	usageKeyboard = 6
	usageKeypad   = 7
	usageGamepad  = 5

	elementInputAxis   = 3
	identityMultiplier = 1.0
	exactIntegerBits   = 53

	nativeEight         = 8
	maxCollectionDepth  = 256
	unitExponentMask    = 15
	unitExponentModulus = 16

	operationRead = "read"
)
