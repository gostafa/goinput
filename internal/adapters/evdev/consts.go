//go:build linux && (amd64 || arm64)

package evdev

const (
	inputDirectory = "/dev/input"
	devicePrefix   = "evdev:"
	keyboardPage   = 0x07
	buttonPage     = 0x09
	desktopPage    = 0x01
	consumerPage   = 0x0c
	wheelDetent    = 120
)
