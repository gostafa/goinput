//go:build darwin && (amd64 || arm64)

package iokit

import (
	"context"
	"errors"
	"testing"
)

func TestQueuedCommandUsesSessionAndReturnsResponse(t *testing.T) {
	t.Parallel()

	result := make(chan response, nativeOne)
	command := requestRecord[int, response]{
		ctx: t.Context(), result: result, pack: packResponse,
		perform: func(state int) (any, error) { return state, nil },
	}
	command.Execute(testAnchorTick)

	reply := <-result
	if reply.err != nil || reply.value != testAnchorTick {
		t.Fatalf("reply = (%v, %v), want (%d, nil)", reply.value, reply.err, testAnchorTick)
	}
}

func TestCanceledQueuedCommandDoesNotExecute(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := make(chan response, nativeOne)
	command := requestRecord[int, response]{
		ctx: ctx, result: result, pack: packResponse,
		perform: func(int) (any, error) {
			t.Error("canceled queued command executed")

			return nil, errors.New("unexpected execution")
		},
	}
	command.Execute(testAnchorTick)

	if reply := <-result; !errors.Is(reply.err, context.Canceled) || reply.value != nil {
		t.Fatalf("canceled reply = (%v, %v)", reply.value, reply.err)
	}
}
