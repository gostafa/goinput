// Package iokit exposes optional macOS IOHID metadata without native handles.
// Obtain MetadataProvider through Device.Extension. All returned values are
// snapshots; registry identifiers and element cookies are session-local.
package iokit
