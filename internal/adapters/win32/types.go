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
	ext "github.com/gostafa/goinput/extensions/win32"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

type (
	commandWait struct {
		command     *command
		contextDone <-chan struct{}
		complete    bool
		err         error
	}
	deviceSnapshot struct {
		devices     []nativeDevice
		diagnostics error
	}
	errorCategory struct {
		native error
		domain error
	}
	inputPacket struct {
		kind   uint32
		device foundation.HANDLE
		body   []byte
		stamp  domain.Timestamp
	}
	keyboardInput struct {
		makeCode   uint16
		flags      uint16
		virtualKey uint16
	}
	mouseInput struct {
		flags   uint16
		buttons uint16
		wheel   int16
		x       int64
		y       int64
		stamp   domain.Timestamp
	}
	reportBatch struct {
		size  uint32
		count uint32
		stamp domain.Timestamp
	}
	hidReportState struct {
		reportID byte
		stamp    domain.Timestamp
		pressed  map[domain.ControlID]bool
	}
	hidBuilder struct {
		descriptor     *descriptor
		caps           hid.HIDP_CAPS
		controls       []domain.Control
		nativeControls []ext.NativeControl
	}
	capabilityRange struct {
		firstUsage uint32
		lastUsage  uint32
		firstIndex uint32
		lastIndex  uint32
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
	backend struct {
		mu            sync.Mutex
		hwnd          foundation.HWND
		wakeEvent     foundation.HANDLE
		closed        bool
		commands      chan *command
		ready         chan error
		done          chan struct{}
		retrier       ports.Retrier
		closeErr      error
		className     string
		stopping      bool // owner-thread fields below
		captures      map[foundation.HANDLE]map[*capture]struct{}
		registrations map[topLevel]int
	}

	command struct {
		ctx   context.Context
		fn    func() error
		reply chan error
		state atomic.Int32
	}

	topLevel struct {
		page  uint16
		usage uint16
	}

	nativeDevice struct {
		handle  foundation.HANDLE
		kind    uint32
		tlc     topLevel
		version uint32
		buttons uint32
		hwheel  bool
		info    domain.DeviceInfo
	}

	capture struct {
		backend   *backend
		device    nativeDevice
		info      domain.DeviceInfo
		caps      domain.Capabilities
		native    ext.Info
		sink      ports.EventSink
		closed    atomic.Bool
		closeOnce sync.Once
		closeErr  error
		hid       *descriptor
		held      map[domain.ControlID]bool
		values    map[domain.ControlID]int64
		buttons   map[byte]map[domain.ControlID]bool
	}

	descriptor struct {
		preparsed []byte
		reportLen uint16
		maxData   uint32
		controls  map[hidIndex]hidControl
		reportIDs map[byte]bool
	}

	hidIndex struct {
		report byte
		index  uint16
	}

	hidControl struct {
		control domain.Control
		native  ext.NativeControl
		button  bool
		hat     bool
	}
)
