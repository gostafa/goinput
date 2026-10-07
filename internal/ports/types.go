package ports

import (
	"context"
	"github.com/gostafa/goinput/internal/domain"
)

// Factory starts a replaceable native session. Failed creation must release all
// partially acquired resources. Startup is never automatically retried.
type Factory func(context.Context, Retrier) (Backend, error)

type Backend interface {
	Discover(context.Context) ([]domain.DeviceInfo, error)
	Open(context.Context, domain.DeviceID, EventSink) (Capture, error)
	Close() error
}

type Capture interface {
	Info() domain.DeviceInfo
	Capabilities() domain.Capabilities
	Extension(any) bool
	Close() error
}

// EventSink is concurrency-safe. Publish never blocks; false means the stream
// has terminated. Native callbacks must copy borrowed data before publishing.
type EventSink interface {
	Publish(domain.Event) bool
	Fail(error)
}

type Provider[T any] interface {
	Get(context.Context) (T, error)
}

// Retrier retries only errors explicitly classified as transient, within a shared
// discovery budget. Implementations must check context before the first attempt.
type Retrier interface {
	Do(context.Context, func(context.Context) error, func(error) bool) error
}
