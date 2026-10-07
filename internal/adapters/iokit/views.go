//go:build darwin && (amd64 || arm64)

package iokit

import (
	"context"

	extension "github.com/gostafa/goinput/extensions/iokit"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

type (
	metadataView[T any] func() T
)

// NativeInfo returns a fresh platform metadata snapshot.
func (view metadataView[T]) NativeInfo() T { return view() }

func (environment *nativeState) backendView(state *backend) ports.Backend {
	view := new(ports.BackendOperations)

	view.Operations.Discover = func(ctx context.Context) ([]domain.DeviceInfo, error) {
		return environment.backendDiscover(ctx, state)
	}
	view.Operations.Open = func(ctx context.Context, id domain.DeviceID, sink ports.EventSink) (ports.Capture, error) {
		return environment.backendOpen(ctx, state, openTarget{id: id, sink: sink})
	}
	view.CloseOperation = func() error { return backendClose(state) }

	return view
}

func (environment *nativeState) captureView(state *capture) ports.Capture {
	view := new(ports.CaptureOperations)

	view.Operations.Info = func() domain.DeviceInfo { return captureInfo(state) }
	view.Operations.Capabilities = func() domain.Capabilities { return captureCapabilities(state) }
	view.Operations.Extension = func(target any) bool { return captureExtension(state, target) }
	view.CloseOperation = func() error { return environment.captureClose(state) }

	return view
}

func captureMetadataView(state *capture) extension.MetadataProvider {
	return metadataView[extension.Metadata](
		func() extension.Metadata { return captureIOKitMetadata(state) },
	)
}

func packResponse(value any, err error) response { return response{value: value, err: err} }

// Execute evaluates a request on the session thread and delivers one response.
func (request *requestRecord[S, R]) Execute(session S) {
	var value any

	err := request.ctx.Err()
	if err == nil {
		value, err = safeCall(func() (any, error) { return request.perform(session) })
	}

	request.result <- request.pack(value, err)
}
