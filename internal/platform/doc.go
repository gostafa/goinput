// Package platform is the composition root's init-only native factory registry.
// OS-tagged adapters register without starting native resources. After package
// initialization the registry is immutable and safe for concurrent reading.
package platform
