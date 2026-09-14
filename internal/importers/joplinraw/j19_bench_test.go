package joplinraw

import (
	"fmt"
	"strings"
	"testing"
)

// J19-A benchmark for the review's claim that `seen := map[string]bool{}`
// without a capacity costs bucket initialisation per note. The variant is a
// copy of notebookPath with a capacity hint, kept here until measured.
func notebookPathSized(id string, folders map[string]parsedItem) string {
	seen := make(map[string]bool, 8)
	parts := []string{}
	for id != "" && !seen[id] {
		seen[id] = true
		folder, ok := folders[id]
		if !ok {
			break
		}
		parts = append([]string{folder.Fields["title"]}, parts...)
		id = folder.Fields["parent_id"]
	}
	return strings.Join(parts, "/")
}

func BenchmarkJ19NotebookPath(b *testing.B) {
	for _, depth := range []int{1, 4, 12} {
		folders := map[string]parsedItem{}
		for index := 0; index < depth; index++ {
			parent := ""
			if index > 0 {
				parent = fmt.Sprintf("folder-%02d", index-1)
			}
			id := fmt.Sprintf("folder-%02d", index)
			folders[id] = parsedItem{ID: id, Fields: map[string]string{"title": "Folder " + id, "parent_id": parent}}
		}
		leaf := fmt.Sprintf("folder-%02d", depth-1)
		if notebookPath(leaf, folders) != notebookPathSized(leaf, folders) {
			b.Fatal("variant disagrees with notebookPath")
		}
		b.Run(fmt.Sprintf("current/depth%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = notebookPath(leaf, folders)
			}
		})
		b.Run(fmt.Sprintf("sized/depth%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = notebookPathSized(leaf, folders)
			}
		})
	}
}
