package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_RunsAllJobs(t *testing.T) {
	p := New(4, 16)

	var counter atomic.Int64
	const N = 100
	for i := 0; i < N; i++ {
		if err := p.Submit(func(ctx context.Context) {
			counter.Add(1)
		}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}

	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := counter.Load(); got != N {
		t.Fatalf("expected %d jobs run, got %d", N, got)
	}
}

func TestPool_SubmitAfterStopFails(t *testing.T) {
	p := New(2, 4)
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.Submit(func(ctx context.Context) {}); !errors.Is(err, ErrPoolStopped) {
		t.Fatalf("expected ErrPoolStopped, got %v", err)
	}
}

func TestPool_TrySubmit(t *testing.T) {
	// Узкая очередь и долгая задача → TrySubmit должен вернуть false.
	p := New(1, 1)

	block := make(chan struct{})
	if err := p.Submit(func(ctx context.Context) { <-block }); err != nil {
		t.Fatal(err)
	}
	// Заполняем очередь.
	if err := p.Submit(func(ctx context.Context) {}); err != nil {
		t.Fatal(err)
	}

	ok, err := p.TrySubmit(func(ctx context.Context) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected TrySubmit to fail when queue is full")
	}

	close(block)
	_ = p.Stop(context.Background())
}

func TestPool_StopTimeoutCancelsJobCtx(t *testing.T) {
	p := New(1, 1)

	jobStarted := make(chan struct{})
	jobCancelled := make(chan struct{})

	if err := p.Submit(func(ctx context.Context) {
		close(jobStarted)
		<-ctx.Done()
		close(jobCancelled)
	}); err != nil {
		t.Fatal(err)
	}

	<-jobStarted

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := p.Stop(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	select {
	case <-jobCancelled:
	case <-time.After(time.Second):
		t.Fatal("job ctx was not cancelled")
	}
}

func TestPool_StopIsIdempotent(t *testing.T) {
	p := New(2, 2)
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop should be no-op, got %v", err)
	}
}

func TestPool_PanicsOnZeroSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = New(0, 1)
}
