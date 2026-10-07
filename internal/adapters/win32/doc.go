// Package win32 adapts Windows Raw Input to the input ports.
//
// Native implementations are available on Windows amd64 and arm64 without cgo.
// A message-only window receives background input. Registration is owned by this
// package for each active top-level usage; an existing application registration
// produces a registration-conflict error rather than being replaced. Applications
// must not replace those registrations while captures are open.
//
// Keyboard and mouse schemas are inferred and incomplete. Other HID schemas
// retain usage/report/collection identity and decode button sets and scalar values
// up to 32 bits. Usage-value arrays remain visible as unsupported capabilities.
// Relative button fields are also marked unsupported; their deltas cannot be
// treated as absolute pressed state. Unknown-scale HID scroll fields use counts.
// Events use host receipt timestamps because Raw Input has no hardware timestamp.
package win32
