//go:build darwin && (amd64 || arm64)

package iokit

import (
	native "github.com/tmc/apple/iokit"
)

func defaultNativeAPI() nativeAPI {
	return nativeAPI{
		managerAPI:         defaultManagerAPI(),
		deviceAPI:          defaultDeviceAPI(),
		elementIdentityAPI: defaultElementIdentityAPI(),
		elementValuesAPI:   defaultElementValuesAPI(),
		valueAPI:           defaultValueAPI(),
		registryAPI:        defaultRegistryAPI(),
	}
}

func defaultManagerAPI() managerAPI {
	return managerAPI{
		IOHIDManagerCreate:                native.IOHIDManagerCreate,
		IOHIDManagerSetDeviceMatching:     native.IOHIDManagerSetDeviceMatching,
		IOHIDManagerCopyDevices:           native.IOHIDManagerCopyDevices,
		IOHIDManagerScheduleWithRunLoop:   native.IOHIDManagerScheduleWithRunLoop,
		IOHIDManagerUnscheduleFromRunLoop: native.IOHIDManagerUnscheduleFromRunLoop,
	}
}

func defaultDeviceAPI() deviceAPI {
	return deviceAPI{
		IOHIDDeviceCreate:                     native.IOHIDDeviceCreate,
		IOHIDDeviceGetService:                 native.IOHIDDeviceGetService,
		IOHIDDeviceGetProperty:                native.IOHIDDeviceGetProperty,
		IOHIDDeviceConformsTo:                 native.IOHIDDeviceConformsTo,
		IOHIDDeviceCopyMatchingElements:       native.IOHIDDeviceCopyMatchingElements,
		IOHIDDeviceGetValue:                   native.IOHIDDeviceGetValue,
		IOHIDDeviceOpen:                       native.IOHIDDeviceOpen,
		IOHIDDeviceClose:                      native.IOHIDDeviceClose,
		IOHIDDeviceRegisterInputValueCallback: nil,
		IOHIDDeviceRegisterRemovalCallback:    nil,
		IOHIDDeviceScheduleWithRunLoop:        native.IOHIDDeviceScheduleWithRunLoop,
		IOHIDDeviceUnscheduleFromRunLoop:      native.IOHIDDeviceUnscheduleFromRunLoop,
	}
}

func defaultElementIdentityAPI() elementIdentityAPI {
	return elementIdentityAPI{
		IOHIDElementGetCookie:         native.IOHIDElementGetCookie,
		IOHIDElementGetType:           native.IOHIDElementGetType,
		IOHIDElementGetUsagePage:      native.IOHIDElementGetUsagePage,
		IOHIDElementGetUsage:          native.IOHIDElementGetUsage,
		IOHIDElementGetName:           native.IOHIDElementGetName,
		IOHIDElementGetParent:         native.IOHIDElementGetParent,
		IOHIDElementGetCollectionType: native.IOHIDElementGetCollectionType,
		IOHIDElementGetLogicalMin:     native.IOHIDElementGetLogicalMin,
		IOHIDElementGetLogicalMax:     native.IOHIDElementGetLogicalMax,
		IOHIDElementGetPhysicalMin:    native.IOHIDElementGetPhysicalMin,
	}
}

func defaultElementValuesAPI() elementValuesAPI {
	return elementValuesAPI{
		IOHIDElementGetPhysicalMax:  native.IOHIDElementGetPhysicalMax,
		IOHIDElementGetUnit:         native.IOHIDElementGetUnit,
		IOHIDElementGetUnitExponent: native.IOHIDElementGetUnitExponent,
		IOHIDElementHasNullState:    native.IOHIDElementHasNullState,
		IOHIDElementIsRelative:      native.IOHIDElementIsRelative,
		IOHIDElementGetReportSize:   native.IOHIDElementGetReportSize,
		IOHIDElementGetReportCount:  native.IOHIDElementGetReportCount,
		IOHIDElementGetReportID:     native.IOHIDElementGetReportID,
		IOHIDElementGetDevice:       native.IOHIDElementGetDevice,
	}
}

func defaultValueAPI() valueAPI {
	return valueAPI{
		IOHIDValueGetElement:      native.IOHIDValueGetElement,
		IOHIDValueGetIntegerValue: native.IOHIDValueGetIntegerValue,
		IOHIDValueGetLength:       native.IOHIDValueGetLength,
		IOHIDValueGetTimeStamp:    native.IOHIDValueGetTimeStamp,
	}
}

func defaultRegistryAPI() registryAPI {
	return registryAPI{
		IORegistryEntryGetRegistryEntryID: native.IORegistryEntryGetRegistryEntryID,
	}
}
