//go:build darwin && (amd64 || arm64)

package iokit

import (
	"sync"
	"sync/atomic"
)

// Trampolines are allocated once because purego cannot release them. The map
// holds Go ownership; native callback context never points into Go memory.
var (
	callbackRegistry     sync.Map
	nextCallbackToken    atomic.Uint64
	symbolOnce           sync.Once
	symbolErr            error
	valueCallback        uintptr
	removalCallback      uintptr
	machAbsoluteTime     func() uint64
	machTimebaseInfo     func(*machTimebase) int32
	registryPath         func(uint32, string, *byte) int32
	timebase             machTimebase
	nativeLibraryHandles []uintptr
	api                  nativeAPI
)
