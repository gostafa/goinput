// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"github.com/gostafa/goinput/internal/ports"
)

const (
	nilContextMessage              = "goinput: nil context"
	discoveryBudget                = ports.DiscoveryBudget
	sessionIdle       sessionState = 16

	sessionStarting sessionState = 17
	sessionRunning  sessionState = 18

	sessionStopping sessionState = 19
	unitStep        sessionState = 1

	emptyQueueSize   = 0
	operationOpen    = "open"
	operationDevices = "devices"
)
