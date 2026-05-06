package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestInMemory_CreateAndGet(t *testing.T) {
	r := NewInMemory()
	ctx := context.Background()

	tree, err := r.Create(ctx, "doc1", "node-A")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tree == nil {
		t.Fatal("nil tree")
	}

	got, err := r.Get(ctx, "doc1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != tree {
		t.Fatal("Get returned different tree pointer")
	}
}

func TestInMemory_GetMissing(t *testing.T) {
	r := NewInMemory()
	_, err := r.Get(context.Background(), "missing")
	if !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("expected ErrDocumentNotFound, got %v", err)
	}
}

func TestInMemory_CreateDuplicate(t *testing.T) {
	r := NewInMemory()
	ctx := context.Background()
	if _, err := r.Create(ctx, "doc1", "n"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(ctx, "doc1", "n"); !errors.Is(err, ErrDocumentExists) {
		t.Fatalf("expected ErrDocumentExists, got %v", err)
	}
}

func TestInMemory_GetOrCreate(t *testing.T) {
	r := NewInMemory()
	ctx := context.Background()

	t1, err := r.GetOrCreate(ctx, "doc1", "n")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := r.GetOrCreate(ctx, "doc1", "n")
	if err != nil {
		t.Fatal(err)
	}
	if t1 != t2 {
		t.Fatal("GetOrCreate must return the same tree on second call")
	}
}

func TestInMemory_ExistsAndDelete(t *testing.T) {
	r := NewInMemory()
	ctx := context.Background()
	if r.Exists(ctx, "x") {
		t.Fatal("should not exist")
	}
	_, _ = r.Create(ctx, "x", "n")
	if !r.Exists(ctx, "x") {
		t.Fatal("should exist")
	}
	if err := r.Delete(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if r.Exists(ctx, "x") {
		t.Fatal("should not exist after delete")
	}
	// Idempotent
	if err := r.Delete(ctx, "x"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestInMemory_ConcurrentGetOrCreate(t *testing.T) {
	r := NewInMemory()
	ctx := context.Background()
	const N = 50

	var wg sync.WaitGroup
	trees := make([]any, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(idx int) {
			defer wg.Done()
			tr, err := r.GetOrCreate(ctx, "shared", "n")
			if err != nil {
				t.Errorf("goroutine: %v", err)
				return
			}
			trees[idx] = tr
		}(i)
	}
	wg.Wait()

	for i := 1; i < N; i++ {
		if trees[i] != trees[0] {
			t.Fatal("GetOrCreate produced different trees under concurrency")
		}
	}
}
