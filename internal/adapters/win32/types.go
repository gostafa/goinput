//go:build windows && (amd64 || arm64)

package win32

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	ext "github.com/gostafa/goinput/extensions/win32"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

type (
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
