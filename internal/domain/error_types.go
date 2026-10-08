// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// ErrorMatcher supports custom errors.Is matching.
	ErrorMatcher interface{ Is(target error) bool }

	// ErrorAssigner supports custom errors.As assignment.
	ErrorAssigner interface{ As(target any) bool }

	// ErrorUnwrapper exposes a single underlying cause.
	ErrorUnwrapper interface{ Unwrap() error }

	// ErrorChildren exposes joined underlying causes.
	ErrorChildren interface{ Unwrap() []error }

	// ErrorReporter formats an error for a caller.
	ErrorReporter interface{ Error() string }
)
