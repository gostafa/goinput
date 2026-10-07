// Package iokit implements the input port with macOS IOHIDManager and IOHIDDevice.
// Its native implementation is selected by build tags for Darwin amd64/arm64.
// Input Monitoring consent is required for protected keyboard devices. Physical
// key transitions are delivered without synthesizing operating-system repeat.
// Wheel resolution is read at Open; reopen after external feature changes.
// Input values wider than 53 reported bits are marked unsupported, preserving
// their logical range and native element metadata without rounding event values.
// Each capture owns an independent IOHIDDevice reference and an opaque callback
// token, so closing a capture does not replace another capture's callbacks.
// Narrow native bridges correct the generated writable-path/context ABIs and
// handle the upstream lowercase framework path on case-sensitive installations.
package iokit
