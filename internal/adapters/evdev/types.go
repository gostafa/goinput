//go:build linux && (amd64 || arm64)

package evdev

import (
	"sync"

	extension "github.com/gostafa/goinput/extensions/evdev"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	native "github.com/holoplot/go-evdev"
)

type (
	backend struct {
		mu        sync.Mutex
		closed    bool
		captures  map[*capture]struct{}
		retrier   ports.Retrier
		closeDone chan struct{}
		closeErr  error
	}

	capture struct {
		owner      *backend
		device     *native.InputDevice
		sink       ports.EventSink
		info       domain.DeviceInfo
		caps       domain.Capabilities
		native     extension.Info
		controls   map[eventCode]domain.Control
		hats       map[int]*hat
		hatCodes   map[eventCode]int
		suppressed map[eventCode]bool
		stop       chan struct{}
		done       chan struct{}
		closeOnce  sync.Once
		closeErr   error
	}

	eventCode struct {
		typeCode native.EvType
		code     native.EvCode
	}

	hat struct {
		id    domain.ControlID
		x, y  int32
		dirty bool
		last  int64
	}
)
