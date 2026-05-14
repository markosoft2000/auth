package argon2

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
	case res := <-uninterruptedCall(ctx, func() []byte {
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
