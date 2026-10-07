//go:build darwin && (amd64 || arm64)

package iokit

const (
	elementInputFirst                = 1
	elementInputLast                 = 4
	elementFeature                   = 257
	elementCollection                = 513
	collectionLogical                = 2
	usageResolutionMultiplier        = 0x48
	ioSuccess                 int32  = 0
	ioNotPermitted            int32  = -536870174 // kIOReturnNotPermitted, 0xe00002e2.
	ioNotPrivileged           int32  = -536870207 // kIOReturnNotPrivileged, 0xe00002c1.
	ioNoDevice                int32  = -536870208 // kIOReturnNoDevice, 0xe00002c0.
	ioNotOpen                 int32  = -536870195 // kIOReturnNotOpen, 0xe00002cd.
	maxNativeElements                = 65536
	maxNativeString                  = 1 << 20
	utf8Encoding              uint32 = 0x08000100
)
