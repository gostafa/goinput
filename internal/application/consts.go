// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import "github.com/gostafa/goinput/internal/ports"

const discoveryBudget = ports.DiscoveryBudget

const (
	sessionIdle sessionState = iota
	sessionStarting
	sessionRunning
	sessionStopping
)
