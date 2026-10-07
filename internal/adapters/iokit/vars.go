//go:build darwin && (amd64 || arm64)

package iokit

import (
	"sync"
	"sync/atomic"

	"github.com/gostafa/goinput/internal/ports"
)

type (
	nativeState struct {
		api                  nativeAPI
		symbolErr            error
		machAbsoluteTime     func() uint64
		machTimebaseInfo     func(*machTimebase) int32
		registryPath         func(uint32, string, *byte) int32
		callbackRegistry     sync.Map
		nativeLibraryHandles []uintptr
		nextCallbackToken    atomic.Uint64
		valueCallback        uintptr
		removalCallback      uintptr
		symbolOnce           sync.Once
		timebase             machTimebase
	}
)

// Factory owns native bindings and callback registries for a shared session.
func Factory() ports.Factory { return new(nativeState).newBackend }
