// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"sync"
	"sync/atomic"
	"unsafe"

	hid "github.com/deploymenttheory/go-bindings-win32/bindings/win32/devices/humaninterfacedevice"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/ui/input"
	ext "github.com/gostafa/goinput/extensions/win32"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

type (
	command                             = commandRecord[*commandState, chan error]
	commandRecord[State any, Reply any] struct {
		commandState State
		reply        Reply
	}

	captureHost                 = captureServices[*keyTables]
	captureServices[Tables any] struct {
		tables     Tables
		call       func(context.Context, func() error) error
		retry      func(context.Context, func(context.Context) error) error
		register   func() error
		unregister func() error
	}

	metadataView[Value any] func() Value

	nativeState = nativeStateRecord[*keyTables]

	nativeStateRecord[Tables any] struct {
		tables       Tables
		windows      sync.Map
		callbackAddr uintptr
		classNumber  atomic.Uint64
		callbackOnce sync.Once
	}
	commandWait = commandWaitRecord[*command]

	commandWaitRecord[Command any] struct {
		err         error
		command     Command
		contextDone <-chan struct{}
		complete    bool
	}
	deviceSnapshot = deviceSnapshotRecord[nativeDevice]

	deviceSnapshotRecord[Device any] struct {
		diagnostics error
		devices     []Device
	}
	errorCategory struct {
		native error
		domain error
	}
	mouseInput = mouseInputRecord[domain.Timestamp]

	mouseInputRecord[Stamp any] struct {
		stamp   Stamp
		x       int64
		y       int64
		flags   uint16
		buttons uint16
		wheel   int16
	}
	reportBatch = reportBatchRecord[domain.Timestamp]

	reportBatchRecord[Stamp any] struct {
		stamp Stamp
		size  uint32
		count uint32
	}
	hidReportState = hidReportStateRecord[domain.Timestamp, domain.ControlID]

	hidReportStateRecord[Stamp any, Control comparable] struct {
		pressed  map[Control]bool
		stamp    Stamp
		reportID byte
	}
	hidBuilder = hidBuilderRecord[*descriptor, hid.HIDP_CAPS, domain.Control, ext.NativeControl]

	hidBuilderRecord[Descriptor any, Caps any, Control any, Native any] struct {
		descriptor     Descriptor
		caps           Caps
		controls       []Control
		nativeControls []Native
	}

	nativeBuffer struct {
		data unsafe.Pointer
		size *uint32
	}
	windowMessage struct {
		hwnd    foundation.HWND
		message uint32
		wParam  foundation.WPARAM
		lParam  foundation.LPARAM
	}
	backend = backendRecord[*nativeState, *command, ports.Retrier, *capture, topLevel]

	backendRecord[Native any, Command any, Retrier any, Capture comparable, Usage comparable] struct {
		retrier       Retrier
		closeErr      error
		ready         chan error
		commands      chan Command
		native        Native
		done          chan struct{}
		captures      map[foundation.HANDLE]map[Capture]struct{}
		registrations map[Usage]int
		className     string
		wakeEvent     foundation.HANDLE
		hwnd          foundation.HWND
		mu            sync.Mutex
		closed        bool
		stopping      bool
	}

	topLevel struct {
		page  uint16
		usage uint16
	}

	nativeDevice = nativeDeviceRecord[topLevel, domain.DeviceInfo]

	nativeDeviceRecord[TopLevel any, Info any] struct {
		info    Info
		tlc     TopLevel
		handle  foundation.HANDLE
		kind    uint32
		version uint32
		buttons uint32
		hwheel  bool
	}

	capture = captureRecord[
		*captureHost,
		nativeDevice,
		domain.DeviceInfo,
		domain.Capabilities,
		ext.Info,
		ports.EventSink,
		*descriptor,
		domain.ControlID,
	]

	captureRecord[
		Backend any,
		Device any,
		Info any,
		Caps any,
		Native any,
		Sink any,
		Descriptor any,
		Control comparable,
	] struct {
		sink      Sink
		closeErr  error
		held      map[Control]bool
		backend   Backend
		buttons   map[byte]map[Control]bool
		values    map[Control]int64
		hid       Descriptor
		info      Info
		caps      Caps
		native    Native
		device    Device
		closeOnce sync.Once
		closed    atomic.Bool
	}

	descriptor = descriptorRecord[hidIndex, hidControl]

	descriptorRecord[Index comparable, Control any] struct {
		controls  map[Index]Control
		reportIDs map[byte]bool
		preparsed []byte
		maxData   uint32
		reportLen uint16
	}

	hidIndex = uint32

	hidControl = hidControlRecord[domain.Control, ext.NativeControl]

	hidControlRecord[Control any, Native any] struct {
		control Control
		native  Native
		button  bool
		hat     bool
	}

	backendOpenArguments = struct {
		sink ports.EventSink
		id   domain.DeviceID
	}
	captureAddScanControlsArguments = struct {
		scans map[uint16]uint16

		page uint16
	}
	captureAbsoluteAxisArguments = struct {
		stamp *domain.Timestamp
		value int64
	}
	captureRelativeAxisArguments = struct {
		stamp *domain.Timestamp
		value int64
	}
	captureProcessHIDControlArguments = struct {
		state *hidReportState
		word  uint32
	}
	captureEmitHIDValueArguments = struct {
		stamp *domain.Timestamp
		value int64
	}
	inventoryEntryArguments = struct {
		snapshot *deviceSnapshot
		item     input.RAWINPUTDEVICELIST
	}
	mouseButtonArguments = struct {
		id    domain.ControlID
		flags uint16
	}

	buttonChangesArguments = struct {
		previous map[domain.ControlID]bool
		event    *domain.Event
	}

	nativeCapability = struct {
		makeControl func(uint32, uint32) hidControl
		words       [eighthValue]uint16
		rangeKind   foundation.BOOLEAN
		alias       foundation.BOOLEAN
	}

	identityHandle = struct{ value foundation.HANDLE }

	capabilityAdapter[Value any] = struct {
		read        func(hid.HIDP_REPORT_TYPE, *Value, *uint16, hid.PHIDP_PREPARSED_DATA) foundation.NTSTATUS
		project     func(*Value) *nativeCapability
		makeControl func(*Value, uint32, uint32) hidControl
	}
)
