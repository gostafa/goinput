// Package goinput provides one read-only input-device API for Linux, macOS, and
// Windows on amd64 and arm64. Native bindings and platform selection stay behind
// internal ports, adapters, and build tags; consumers need no runtime OS checks.
// Public types are declared in this package. Managers and devices delegate to
// private implementations without exposing internal types in their API.
//
// Create a manager with New(Options{}), enumerate with Manager.Devices, open an
// endpoint with Manager.Open, then call Device.Read with a context. Always close
// the device and manager. New allocates no native resources. Managers share one
// native session; its last manager lease shuts it down, and later managers can
// start another session. Devices returns accessible endpoints together with any
// partial discovery diagnostics. An empty accessible list is valid.
// If a native event loop fails, close all managers leasing that session before
// creating a new session. Live managers do not migrate or reconnect automatically.
//
// # Portable behavior
//
// Device IDs identify current OS endpoints and can be reused after unplugging.
// A physical device may expose several endpoints or top-level collections. Names,
// paths, and VID/PID pairs are not unique identifiers. Missing strings, nil vendor
// or product IDs, unknown enum values, and incomplete capability snapshots remain
// explicit. Metadata snapshots are defensive copies. Duplicate HID usages retain
// distinct device-local control IDs. Gamepad button ordinals do not promise a
// physical south/east/north/west layout.
//
// Keys describe physical controls, not composed text or keyboard layout. Digital
// events carry 0/1 and press/release/observed-repeat actions. Absolute axes retain
// logical positions and ranges; relative axes are deltas. Control.Normalize
// explicitly scales valid absolute-axis values to [0,1], without inferring a
// center or deadzone. Scrolling is in fractional detents when scaling is known,
// otherwise counts; consult Control.Unit. Conventional hats use HatNeutral (-1)
// and directions 0..7 clockwise from north. Unsupported hat encodings remain
// logical axes. HID units/ranges are not universal physical measurements.
//
// Read cancellation cancels only that read. Concurrent readers consume each
// queued event once; they are not independent subscriptions. Separate Open calls
// create independent streams. A bounded queue never blocks a native callback.
// Queue overflow and native event loss terminate the affected stream, discard
// pending events, and report ErrEventLoss. Reopen explicitly; no lost motion,
// synthetic releases, or automatic reconnects are invented. Close is idempotent,
// unblocks readers, and waits for native capture teardown. ErrDisconnected means
// the endpoint disappeared. Use errors.Is to classify portable errors.
//
// Event timestamps always distinguish event time and host receipt time. Receipt
// time retains Go's monotonic component; native/estimated wall times do not
// promise monotonicity, hardware latency, or order across devices. There is no
// portable native report/frame boundary in the event API.
//
// # Platform details
//
// Linux uses read-only evdev endpoints and requires access to /dev/input/event*.
// Capabilities may be incomplete because the binding suppresses some ioctl errors.
// HID mappings are inferred from evdev semantics and cannot recover the original
// HID descriptor perfectly. Native timestamps use the default realtime clock.
// SYN_DROPPED terminates the stream. Repeat events are observed from EV_KEY.
//
// macOS uses IOHID devices/elements, requires Input Monitoring permission for
// protected input, and never requests consent automatically. IOHID does not
// provide OS keyboard repeat; repeat is unavailable. Native Mach timestamps are
// aligned to a host-clock anchor and marked estimated. Optional Path may be empty.
// HID scroll resolution multipliers are read at open; reopen after external
// feature changes. An unreadable multiplier leaves count units.
//
// Windows uses Raw Input in a background message window without suppressing
// legacy messages. Registrations are process-wide per top-level usage: active
// streams require ownership, and conflicting host registrations fail with
// ErrRegistrationConflict. Arbitrary host Raw Input clients cannot be coordinated
// transparently. Keyboard/mouse capabilities and names/IDs may be inferred or
// incomplete. Scalar HID inputs up to 32 bits are supported; usage-value arrays
// are explicitly unsupported. Raw Input supplies no hardware timestamp, so event
// times are receipt times. RDP devices may not appear in Raw Input enumeration.
//
// Native discovery retries only explicitly transient query failures, with four
// attempts and a shared 500 ms discovery budget. Synchronous native functions can
// outlast cancellation until they return. Capture startup, registration, reads,
// delivery, and close are not retried.
//
// Optional typed native metadata lives in the extensions subpackages. Obtain it
// through a Device's ExtensionProvider interface, passing a pointer to a supported
// extension struct. Unknown targets return false. No binding types cross the API.
//
// Input injection, outputs, LEDs, force feedback, exclusive grabs, text input,
// hotplug subscriptions, and automatic event-loss recovery are outside this API.
package goinput
