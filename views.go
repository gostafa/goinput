// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"context"

	"github.com/gostafa/goinput/internal/application"
)

func newManagerView(impl managerImpl) *Manager {
	view := new(Manager)

	view.Operations.Devices = func(ctx context.Context) ([]DeviceInfo, error) { return managerDevices(ctx, impl) }
	view.Operations.Open = func(ctx context.Context, id DeviceID) (Device, error) { return managerOpen(ctx, impl, id) }
	view.CloseOperation = func() error { return managerClose(impl) }

	return view
}

func newDeviceView(impl application.Device) *device {
	view := new(device)

	view.Operations.Info = func() DeviceInfo { return deviceInfo(impl) }
	view.Operations.Capabilities = func() Capabilities { return deviceCapabilities(impl) }
	view.Operations.Read = func(ctx context.Context) (Event, error) { return deviceRead(ctx, impl) }
	configureDeviceClose(view, impl)

	return view
}

func configureDeviceClose(view *device, impl application.Device) {
	view.Operations.Extension = func(target any) bool { return deviceExtension(impl, target) }
	view.CloseOperation = func() error { return deviceClose(impl) }
}
