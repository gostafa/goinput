//go:build darwin && (amd64 || arm64)

package iokit

import (
	"sync"
	"sync/atomic"
)

// Trampolines are allocated once because purego cannot release them. The map
// holds Go ownership; native callback context never points into Go memory.
var callbackRegistry sync.Map
var nextCallbackToken atomic.Uint64
var symbolOnce sync.Once
var symbolErr error
var valueCallback uintptr
var removalCallback uintptr
var machAbsoluteTime func() uint64
var machTimebaseInfo func(*machTimebase) int32
var registryPath func(uint32, string, *byte) int32
var timebase machTimebase
var nativeLibraryHandles []uintptr
var api nativeAPI
