package shell

import (
	"context"
	"errors"
)

// stream owns one cancellable backend subscription whose values are applied on
// the UI goroutine. Events queued by a stopped subscription carry an older
// epoch and are dropped.
type stream[T any] struct {
	events          chan streamEvent[T]
	cancel          context.CancelFunc
	epoch           uint64
	started, failed bool
}

type streamEvent[T any] struct {
	epoch uint64
	value T
	err   error
}

func newStream[T any]() *stream[T] { return &stream[T]{events: make(chan streamEvent[T], 64)} }

func (st *stream[T]) stop() {
	if !st.started {
		return
	}
	if st.cancel != nil {
		st.cancel()
	}
	st.cancel = nil
	st.epoch++
	st.started, st.failed = false, false
}

func (st *stream[T]) start(invalidate func(), watch func(context.Context, func(T)) error) {
	ctx, cancel := context.WithCancel(context.Background())
	st.started, st.cancel = true, cancel
	epoch, events := st.epoch, st.events
	publish := func(e streamEvent[T]) {
		select {
		case events <- e:
			invalidate()
		case <-ctx.Done():
		}
	}
	go func() {
		err := watch(ctx, func(v T) { publish(streamEvent[T]{epoch: epoch, value: v}) })
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			// A stream that ends on its own is not a live subscription.
			err = errors.New("订阅已结束 / Subscription ended")
		}
		publish(streamEvent[T]{epoch: epoch, err: err})
	}()
}

// drain applies the current subscription's queued events.
func (st *stream[T]) drain(apply func(T), fail func(error)) {
	for {
		select {
		case e := <-st.events:
			if e.epoch != st.epoch {
				continue
			}
			if e.err != nil {
				st.failed = true
				fail(e.err)
			} else {
				apply(e.value)
			}
		default:
			return
		}
	}
}
