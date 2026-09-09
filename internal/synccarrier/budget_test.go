package synccarrier

import (
	"context"
	"errors"
	"testing"
)

type budgetTestCarrier struct{ values map[string][]byte }

func (c *budgetTestCarrier) Initialize(context.Context) error { return nil }
func (c *budgetTestCarrier) Publish(_ context.Context, _ Class, name string, artifact []byte) (string, error) {
	if c.values == nil {
		c.values = map[string][]byte{}
	}
	c.values[name] = append([]byte(nil), artifact...)
	return name, nil
}
func (c *budgetTestCarrier) Namespaces(context.Context) ([]string, error)          { return []string{"ns"}, nil }
func (c *budgetTestCarrier) List(context.Context, string, Class) ([]string, error) { return nil, nil }
func (c *budgetTestCarrier) Read(_ context.Context, _ string, _ Class, name string) ([]byte, error) {
	return append([]byte(nil), c.values[name]...), nil
}
func (c *budgetTestCarrier) Remove(context.Context, Class, string) error { return nil }
func (c *budgetTestCarrier) Namespace() string                           { return "ns" }

func TestG15BudgetCarrierBoundsWritesAndAccountsOneReadOvershoot(t *testing.T) {
	inner := &budgetTestCarrier{values: map[string][]byte{"large": make([]byte, 8)}}
	budget := NewBudgetCarrier(inner, 10)
	if _, err := budget.Publish(context.Background(), ClassEnvelope, "small", make([]byte, 4)); err != nil {
		t.Fatal(err)
	}
	if _, err := budget.Publish(context.Background(), ClassEnvelope, "too-much", make([]byte, 7)); !errors.Is(err, ErrByteBudget) {
		t.Fatalf("write budget error = %v", err)
	}
	if _, err := budget.Read(context.Background(), "ns", ClassEnvelope, "large"); !errors.Is(err, ErrByteBudget) {
		t.Fatalf("read budget error = %v", err)
	}
	if budget.Used() != 12 {
		t.Fatalf("used = %d, want actual transferred bytes 12", budget.Used())
	}
}
