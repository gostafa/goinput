// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"

	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
)

// ManagerView exposes operations while keeping mutable manager state private.
func ManagerView(
	state *Manager,
) *ports.ManagerOperations[domain.DeviceInfo, domain.DeviceID, Device] {
	view := new(ports.ManagerOperations[domain.DeviceInfo, domain.DeviceID, Device])

	view.Operations.Devices = func(ctx context.Context) ([]domain.DeviceInfo, error) {
		return managerDevices(ctx, state)
	}
	view.Operations.Open = func(ctx context.Context, id domain.DeviceID) (Device, error) {
		return managerOpen(ctx, state, id)
	}
	view.CloseOperation = func() error { return managerClose(state) }

	return view
}

func streamView(
	state *stream,
) *ports.DeviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event] {
	view := new(ports.DeviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event])

	view.Operations.Info = func() domain.DeviceInfo { return streamInfo(state) }
	view.Operations.Capabilities = func() domain.Capabilities { return streamCapabilities(state) }
	view.Operations.Read = func(ctx context.Context) (domain.Event, error) { return streamRead(ctx, state) }
	configureDeviceClose(view, state)

	return view
}

func leasedSinkView(state leasedSink) ports.EventSink {
	return sinkView(state, leasedSinkPublish, leasedSinkFail)
}

func streamSinkView(state *stream) ports.EventSink {
	return sinkView(state, streamPublish, streamFail)
}

func configureDeviceClose(
	view *ports.DeviceOperations[domain.DeviceInfo, domain.Capabilities, domain.Event],
	state *stream,
) {
	view.Operations.Extension = func(target any) bool { return streamExtension(state, target) }
	view.CloseOperation = func() error { return streamClose(state) }
}

func sinkView[S any](
	state S,
	publish func(S, *domain.Event) bool,
	fail func(S, error),
) *ports.SinkOperations[domain.Event] {
	view := new(ports.SinkOperations[domain.Event])

	view.Operations.Publish = func(event *domain.Event) bool { return publish(state, event) }
	view.Operations.Fail = func(err error) { fail(state, err) }

	return view
}
