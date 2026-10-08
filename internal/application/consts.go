// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"github.com/gostafa/goinput/internal/ports"
)

const (
	leaseErrorFormat              = "getLease: %w"
	discoveryBudget               = ports.DiscoveryBudget
	unitStep                      = 1
	emptyQueueSize                = 0
	operationOpen                 = "open"
	operationDevices              = "devices"
	sessionIdle      sessionState = 16
	sessionStarting  sessionState = 17
	sessionRunning   sessionState = 18
	sessionStopping  sessionState = 19
)
