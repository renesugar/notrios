package joplinraw

import (
	"fmt"
	"testing"
)

// TestBoundedByItemBytes states the window rule: all the items when they fit the
// budget, as many as fit when they do not, and never fewer than one so an item
// at the size limit still imports (v1.0 J38-C).
func TestBoundedByItemBytes(t *testing.T) {
	cases := []struct {
		name  string
		sizes []int64
		want  int
	}{
		{"small items all fit", []int64{10, 10, 10, 10}, 4},
		{"none", nil, 0},
		{"one item past the budget stands alone", []int64{importBatchByteBudget + 1, 10, 10}, 1},
		{"one item at the budget stands alone", []int64{importBatchByteBudget, 10}, 1},
		{"the budget closes a window early", []int64{importBatchByteBudget / 2, importBatchByteBudget / 2, 10}, 2},
		{"three large items are three windows", []int64{importBatchByteBudget, importBatchByteBudget, importBatchByteBudget}, 1},
		{"a small item before a large one", []int64{10, importBatchByteBudget, 10}, 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			items := make([]inventoryItem, 0, len(testCase.sizes))
			for index, size := range testCase.sizes {
				items = append(items, inventoryItem{SizeBytes: size, RelativePath: fmt.Sprintf("item-%d.md", index)})
			}
			if got := boundedByItemBytes(items); got != testCase.want {
				t.Errorf("boundedByItemBytes(%v) = %d, want %d", testCase.sizes, got, testCase.want)
			}
		})
	}
	if importBatchByteBudget != int64(maxRAWItemSize) {
		t.Errorf("the budget should be the size one item may reach: %d against %d",
			importBatchByteBudget, maxRAWItemSize)
	}
}
