// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"sync"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	deviceview "github.com/gostafa/goinput/internal/ports/device"
	managerview "github.com/gostafa/goinput/internal/ports/manager"
	sinkview "github.com/gostafa/goinput/internal/ports/sink"
)

type (
	deviceOperations[I, C, E any]   = deviceview.Operations[I, C, E]
	managerOperations[I, ID, D any] = managerview.Operations[I, ID, D]
	sinkOperations[E any]           = sinkview.Operations[E]

	// Device is a concurrency-safe event consumer. Concurrent readers share one
	// ordered queue: events are consumed once, rather than broadcast to readers.
	Device interface {
		ports.InfoProvider[domain.DeviceInfo]
		ports.CapabilitiesProvider[domain.Capabilities]
		ports.EventReader[domain.Event]
		ports.Closer
	}

	// ExtensionProvider queries typed native metadata without native types in Device.
	// The target must be a non-nil pointer to a supported extension struct.
	ExtensionProvider interface{ ports.ExtensionProvider }

	// Manager owns its captures and a lazily acquired shared native-session lease.
	// Construct with NewManager; the zero value is not usable. Do not copy a manager.
	Manager = managerRecord[ports.Provider[*Coordinator], *lease, *stream, domain.Options]

	managerRecord[P, L any, D comparable, O any] struct {
		options   O
		closeErr  error
		onClose   func(context.CancelCauseFunc) func() bool
		lease     L
		provider  P
		devices   map[D]struct{}
		leaseGate chan struct{}
		cancel    context.CancelCauseFunc
		closeDone chan struct{}
		ops       sync.WaitGroup
		closeOnce sync.Once
		mu        sync.Mutex
		closed    bool
	}

	sessionState uint8

	// Coordinator is process-lived Go state. Only its replaceable session owns
	// native resources. Successful singleton initialization never caches a handle.
	Coordinator = coordinatorRecord[ports.Retrier, ports.Backend, ports.Factory, sessionState]

	coordinatorRecord[R, B, F any, S ~uint8] struct {
		retrier    R
		backend    B
		factory    F
		state      S
		changed    chan struct{}
		refs       int
		generation uint64
		mu         sync.Mutex
	}

	lease = leaseRecord[ports.Backend, *Coordinator]

	leaseRecord[B, C any] struct {
		backend     B
		err         error
		coordinator C
		generation  uint64
		once        sync.Once
	}

	leasedSink = leasedSinkRecord[*lease, ports.EventSink]

	leasedSinkRecord[L, S any] struct {
		lease L
		sink  S
	}

	stream = streamRecord[ports.Capture, domain.DeviceID, domain.DeviceInfo, domain.Event, domain.Capabilities]

	streamRecord[C, ID, I, E, Caps any] struct {
		closeErr  error
		capture   C
		terminal  error
		caps      Caps
		info      I
		id        ID
		done      chan struct{}
		closeDone chan struct{}
		release   func()
		notify    chan struct{}
		queue     []E
		size      int
		head      int
		closeOnce sync.Once
		mu        sync.Mutex
	}
	// acquisition distinguishes a ready lease from a pending session transition.
	acquisition = acquisitionRecord[*lease]

	acquisitionRecord[Lease any] struct {
		lease   Lease
		changed <-chan struct{}
	}
)
