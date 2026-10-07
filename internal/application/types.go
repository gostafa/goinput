package application

import (
	"context"
	"sync"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

// Device is a concurrency-safe event consumer. Concurrent readers share one
// ordered queue: events are consumed once, rather than broadcast to readers.
type Device interface {
	Info() domain.DeviceInfo
	Capabilities() domain.Capabilities
	Read(context.Context) (domain.Event, error)
	Close() error
}

// ExtensionProvider queries typed native metadata without native types in Device.
// The target must be a non-nil pointer to a supported extension struct.
type ExtensionProvider interface{ Extension(target any) bool }

// Manager owns its captures and a lazily acquired shared native-session lease.
// Construct with NewManager; the zero value is not usable. Do not copy a manager.
type Manager struct {
	mu        sync.Mutex
	ops       sync.WaitGroup
	closed    bool
	devices   map[*stream]struct{}
	options   domain.Options
	provider  ports.Provider[*Coordinator]
	lease     *lease
	leaseGate chan struct{}
	ctx       context.Context
	cancel    context.CancelCauseFunc
	closeOnce sync.Once
	closeDone chan struct{}
	closeErr  error
}

type sessionState uint8

// Coordinator is process-lived Go state. Only its replaceable session owns
// native resources. Successful singleton initialization never caches a handle.
type Coordinator struct {
	mu         sync.Mutex
	state      sessionState
	changed    chan struct{}
	factory    ports.Factory
	retrier    ports.Retrier
	backend    ports.Backend
	refs       int
	generation uint64
}

type lease struct {
	coordinator *Coordinator
	backend     ports.Backend
	generation  uint64
	once        sync.Once
	err         error
}

type leasedSink struct {
	lease *lease
	sink  ports.EventSink
}

type stream struct {
	mu        sync.Mutex
	owner     *Manager
	id        domain.DeviceID
	info      domain.DeviceInfo
	caps      domain.Capabilities
	capture   ports.Capture
	queue     []domain.Event
	head      int
	size      int
	notify    chan struct{}
	done      chan struct{}
	terminal  error
	closeOnce sync.Once
	closeDone chan struct{}
	closeErr  error
}
