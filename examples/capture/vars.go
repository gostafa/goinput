// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"errors"
)

var errUsage = errors.New("usage: capture [-cycles n] [-managers n] [-duration 2s] [device-id]")
