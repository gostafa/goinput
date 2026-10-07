// Package singleton adapts the singleton dependency to the provider port.
// Only inert process-lived coordinators belong in these providers, not native
// sessions: a successfully initialized singleton cannot be reset or closed.
package singleton
