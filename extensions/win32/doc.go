// Package win32 exposes optional Windows Raw Input metadata.
//
// Ask a device for Metadata through its Extension method. Native handles are
// identifiers borrowed from Windows; callers must not close or retain them after
// the device disconnects. This package performs no native calls and is portable.
package win32
