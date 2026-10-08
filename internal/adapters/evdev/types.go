// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build linux && (amd64 || arm64)

package evdev

import (
	"os"
	"sync"

	extension "github.com/gostafa/goinput/extensions/evdev"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	backendview "github.com/gostafa/goinput/internal/ports/backend"
	native "github.com/holoplot/go-evdev"
)

type (
	nativeOpener      = func(string, int) (*native.InputDevice, error)
	backendOperations = backendview.Operations[domain.DeviceInfo, domain.DeviceID, ports.EventSink, ports.Capture]
	// Native calls are isolated for testing ioctl failures without hardware.
	environment = struct {
		readDirectory func(string) ([]os.DirEntry, error)
		openDevice    func(string, int) (*device, error)
	}
	device = struct {
		path             func() string
		name             func() (string, error)
		inputID          func() (native.InputID, error)
		uniqueID         func() (string, error)
		physicalLocation func() (string, error)
		properties       func() []native.EvProp
		absInfos         func() (map[native.EvCode]native.AbsInfo, error)
		capableTypes     func() []native.EvType
		capableEvents    func(native.EvType) []native.EvCode
		nonBlock         func() error
		readOne          func() (*native.InputEvent, error)
		release          func() error
	}
	backend                              = backendState[*capture, ports.Retrier, environment]
	backendState[C comparable, R, E any] struct {
		retrier     R
		closeErr    error
		environment E
		captures    map[C]struct{}
		closeDone   chan struct{}
		mu          sync.Mutex
		closed      bool
	}
	capture = captureState[
		func(),
		*device,
		ports.EventSink,
		domain.DeviceInfo,
		domain.Capabilities,
		extension.Info,
		domain.Control,
		*hat,
		eventCode,
	]
	captureState[O, D, S, I, C, N, Control, H any, Code comparable] struct {
		sink       S
		closeErr   error
		suppressed map[Code]bool
		controls   map[Code]Control
		hats       map[int]H
		hatCodes   map[Code]int
		unregister O
		stop       chan struct{}
		done       chan struct{}
		device     D
		info       I
		native     N
		caps       C
		closeOnce  sync.Once
	}
	eventCode = struct {
		typeCode native.EvType
		code     native.EvCode
	}
	hat = struct {
		id    domain.ControlID
		x, y  int32
		dirty bool
		last  int64
	}
	description = struct {
		info   domain.DeviceInfo
		caps   domain.Capabilities
		native extension.Info
	}
	openRequest = struct {
		sink ports.EventSink
		id   domain.DeviceID
	}
	controlRequest = struct {
		axes      map[native.EvCode]native.AbsInfo
		eventType native.EvType
		code      native.EvCode
	}
	wheelPair           = struct{ low, high native.EvCode }
	codeRange           = struct{ first, last native.EvCode }
	metadataView[T any] func() T
	eventTypeRequest    = struct {
		keys      map[native.EvCode]bool
		eventType native.EvType
	}
	discoveryResult = struct {
		infos       *[]domain.DeviceInfo
		diagnostics *[]error
	}
	deviceOpenRequest = struct {
		request *openRequest
		path    string
	}
	usageLookup = func(native.EvCode) domain.Usage
)
