// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// InfoProvider supplies an immutable endpoint snapshot.
	InfoProvider[I any] interface{ Info() I }
	// CapabilitiesProvider supplies an immutable control-schema snapshot.
	CapabilitiesProvider[C any] interface{ Capabilities() C }
	// MetadataProvider supplies platform-specific metadata without owning handles.
	MetadataProvider[M any] interface{ NativeInfo() M }
	// ExtensionProvider queries an optional typed native extension.
	ExtensionProvider interface{ Extension(target any) bool }
	// NativeControlProvider supplies native control mappings as a fresh slice.
	NativeControlProvider[C any] interface{ NativeControls() []C }
)
