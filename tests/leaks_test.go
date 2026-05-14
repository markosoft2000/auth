package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestUninterruptedCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	select {
	case res := <-uninterruptedCall[[]byte](ctx, func() []byte {
		defer wg.Done()

		time.Sleep(time.Second * 2)

		return []byte("ss12")
	}):
		t.Log("computed", res)
		return
	case <-ctx.Done():
		t.Log("Context timed out as expected after 1s")
		wg.Wait()
		return
	}
}

// uninterruptedCall fuc is a wrapper for calls without ctx (slow, long calls)
func uninterruptedCall[T any](ctx context.Context, f func() T) <-chan T {
	ch := make(chan T, 1)

	go func() {
		defer close(ch)

		select {
		case ch <- f():
		case <-ctx.Done():
			return
		}
	}()

	return ch
}
