package synccarrier

import (
	"context"
	"errors"
	"sync"
)

// ErrByteBudget marks a bounded sync attempt that must stop before doing more
// transport work. Retrying replans from durable vectors/checkpoints.
var ErrByteBudget = errors.New("sync byte budget exhausted")

// BudgetCarrier accounts artifact payload bytes without learning or exposing
// their plaintext. A read can exceed the remaining allowance by at most one
// already-bounded carrier artifact because Carrier does not expose trusted size
// metadata before the verified read.
type BudgetCarrier struct {
	inner  Carrier
	budget int64

	mu   sync.Mutex
	used int64
}

func NewBudgetCarrier(inner Carrier, budget int64) *BudgetCarrier {
	return &BudgetCarrier{inner: inner, budget: budget}
}

func (b *BudgetCarrier) Initialize(ctx context.Context) error { return b.inner.Initialize(ctx) }
func (b *BudgetCarrier) Namespaces(ctx context.Context) ([]string, error) {
	return b.inner.Namespaces(ctx)
}
func (b *BudgetCarrier) List(ctx context.Context, namespace string, class Class) ([]string, error) {
	return b.inner.List(ctx, namespace, class)
}
func (b *BudgetCarrier) Namespace() string { return b.inner.Namespace() }
func (b *BudgetCarrier) Remove(ctx context.Context, class Class, name string) error {
	return b.inner.Remove(ctx, class, name)
}

func (b *BudgetCarrier) Publish(ctx context.Context, class Class, name string, artifact []byte) (string, error) {
	if !b.reserve(int64(len(artifact)), false) {
		return "", ErrByteBudget
	}
	written, err := b.inner.Publish(ctx, class, name, artifact)
	if err != nil {
		b.release(int64(len(artifact)))
	}
	return written, err
}

func (b *BudgetCarrier) Read(ctx context.Context, namespace string, class Class, name string) ([]byte, error) {
	artifact, err := b.inner.Read(ctx, namespace, class, name)
	if err != nil {
		return nil, err
	}
	if !b.reserve(int64(len(artifact)), true) {
		return nil, ErrByteBudget
	}
	return artifact, nil
}

func (b *BudgetCarrier) Used() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

func (b *BudgetCarrier) reserve(size int64, accountOvershoot bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if size < 0 || b.budget <= 0 || size > b.budget-b.used {
		if accountOvershoot && size > 0 {
			b.used += size
		}
		return false
	}
	b.used += size
	return true
}

func (b *BudgetCarrier) release(size int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.used -= size
	if b.used < 0 {
		b.used = 0
	}
}
