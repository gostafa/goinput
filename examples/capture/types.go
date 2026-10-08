// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"flag"
	"io"
	"log/slog"
	"time"
)

type (
	captureFlags = struct {
		flags    *flag.FlagSet
		cycles   *int
		managers *int
		duration *time.Duration
	}

	captureOptions struct {
		output   io.Writer
		logger   *slog.Logger
		args     []string
		cycles   int
		managers int
		duration time.Duration
	}
)
