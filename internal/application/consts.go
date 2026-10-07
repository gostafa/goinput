package application

import "github.com/gostafa/goinput/internal/ports"

const discoveryBudget = ports.DiscoveryBudget

const (
	sessionIdle sessionState = iota
	sessionStarting
	sessionRunning
	sessionStopping
)
