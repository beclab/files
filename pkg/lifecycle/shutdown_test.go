package lifecycle

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoordinatorRunsHooksInReverseOrder(t *testing.T) {
	c := New()
	var order []string
	c.Add("first", 0, func(context.Context) error {
		order = append(order, "first")
		return nil
	})
	c.Add("second", 0, func(context.Context) error {
		order = append(order, "second")
		return nil
	})
	c.Add("third", 0, func(context.Context) error {
		order = append(order, "third")
		return nil
	})

	c.Run(context.Background())

	got := []string{}
	got = append(got, order...)
	want := []string{"third", "second", "first"}
	if len(got) != len(want) {
		t.Fatalf("expected %v hooks, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: want %v got %v", i, want, got)
		}
	}
}

func TestCoordinatorPerHookTimeoutDoesNotBlockOthers(t *testing.T) {
	c := New()
	var ranSecond atomic.Bool
	c.Add("slow-deps", 0, func(context.Context) error {
		ranSecond.Store(true)
		return nil
	})
	c.Add("slow", 20*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-time.After(500 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	start := time.Now()
	c.Run(context.Background())
	elapsed := time.Since(start)

	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected per-hook timeout to bound total time, took %s", elapsed)
	}
	if !ranSecond.Load() {
		t.Fatalf("expected earlier-registered hook to still run after slow hook timeout")
	}
}

func TestCoordinatorParentDeadlineStopsChain(t *testing.T) {
	c := New()
	var ran atomic.Int32
	c.Add("a", 0, func(context.Context) error {
		ran.Add(1)
		return nil
	})
	c.Add("b", 0, func(context.Context) error {
		ran.Add(1)
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	c.Run(ctx)

	if ran.Load() == 2 {
		t.Fatalf("expected parent deadline to skip earlier-registered hook")
	}
}

func TestCoordinatorPanicIsIsolated(t *testing.T) {
	c := New()
	var ran atomic.Bool
	c.Add("survivor", 0, func(context.Context) error {
		ran.Store(true)
		return nil
	})
	c.Add("boom", 0, func(context.Context) error {
		panic("boom")
	})

	c.Run(context.Background())

	if !ran.Load() {
		t.Fatalf("expected survivor hook to run after panicking hook")
	}
}

func TestCoordinatorErrorDoesNotAbortChain(t *testing.T) {
	c := New()
	var ran atomic.Bool
	sentinel := errors.New("nope")
	c.Add("survivor", 0, func(context.Context) error {
		ran.Store(true)
		return nil
	})
	c.Add("err", 0, func(context.Context) error {
		return sentinel
	})

	c.Run(context.Background())
	if !ran.Load() {
		t.Fatalf("expected survivor hook to run after error hook")
	}
}

func TestCoordinatorNilStopIgnored(t *testing.T) {
	c := New()
	c.Add("nil", 0, nil)
	if len(c.hooks) != 0 {
		t.Fatalf("expected nil stop to be dropped, got %d hooks", len(c.hooks))
	}
}
