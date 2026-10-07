//go:build windows && (amd64 || arm64)

package win32

const (
	messageWake         = 0x8000 + 0x473
	messageInput        = 0x00ff
	messageDeviceChange = 0x00fe
	deviceMouse         = 0
	deviceKeyboard      = 1
	deviceHID           = 2
	deviceRemoved       = 2
	registrationFlags   = 0x00000100 | 0x00002000 // INPUTSINK | DEVNOTIFY
	registrationRemove  = 0x00000001
	maxNativeBuffer     = 16 << 20
	maxDevices          = 1 << 16
	maxUint32           = ^uint32(0)
)
