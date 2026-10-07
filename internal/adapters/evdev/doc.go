// Package evdev adapts Linux evdev devices to the input backend ports.
// It supports read-only capture on Linux amd64 and arm64. HID usages are
// inferred from evdev semantics; original HID report identities are unavailable.
package evdev
