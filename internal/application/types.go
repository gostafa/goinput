// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

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
	provider  ports.Provider[*Coordinator]
	closeErr  error
	ctx       context.Context
	lease     *lease
	devices   map[*stream]struct{}
	leaseGate chan struct{}
	cancel    context.CancelCauseFunc
	closeDone chan struct{}
	ops       sync.WaitGroup
	options   domain.Options
	closeOnce sync.Once
	mu        sync.Mutex
	closed    bool
}

type sessionState uint8

// Coordinator is process-lived Go state. Only its replaceable session owns
// native resources. Successful singleton initialization never caches a handle.
type Coordinator struct {
	retrier    ports.Retrier
	backend    ports.Backend
	changed    chan struct{}
	factory    ports.Factory
	refs       int
	generation uint64
	mu         sync.Mutex
	state      sessionState
}

type lease struct {
	backend     ports.Backend
	err         error
	coordinator *Coordinator
	generation  uint64
	once        sync.Once
}

type leasedSink struct {
	lease *lease
	sink  ports.EventSink
}

type stream struct {
	closeErr  error
	capture   ports.Capture
	terminal  error
	notify    chan struct{}
	owner     *Manager
	closeDone chan struct{}
	done      chan struct{}
	id        domain.DeviceID
	info      domain.DeviceInfo
	queue     []domain.Event
	caps      domain.Capabilities
	size      int
	head      int
	closeOnce sync.Once
	mu        sync.Mutex
}
