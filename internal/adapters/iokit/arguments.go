//go:build darwin && (amd64 || arm64)

package iokit

import (
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	cf "github.com/tmc/apple/corefoundation"
	native "github.com/tmc/apple/iokit"
)

type (
	openFailure struct {
		err       error
		recovered any
	}
	openTarget struct {
		sink ports.EventSink
		id   domain.DeviceID
	}
	deviceSearch struct {
		id      domain.DeviceID
		devices []native.IOHIDDeviceRef
	}
	captureSetup struct {
		resources *deviceResources
		sink      ports.EventSink
	}
	deviceInventory struct {
		devices []native.IOHIDDeviceRef
		set     cf.CFSetRef
	}
	deviceReference struct{ ref native.IOHIDDeviceRef }
)
