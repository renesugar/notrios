package obsidian

import (
	"fmt"
	"testing"
)

// F9 says building the link namespace costs on a vault where many notes share
// a name, because appendUnique scans what is already under that name. This
// measures it at the collision corpus's shape: 5,000 notes called index.md,
// 50 aliases shared by 100 notes each (v1.0 J32-F).
func benchmarkNamespace(b *testing.B, notes, aliasGroups int) {
	inv := inventory{Notes: make([]vaultFile, 0, notes)}
	for index := 0; index < notes; index++ {
		inv.Notes = append(inv.Notes, vaultFile{
			RelPath:  fmt.Sprintf("area-%02d/topic-%05d/index.md", index%50, index),
			Title:    fmt.Sprintf("Topic %d", index),
			Aliases:  []string{fmt.Sprintf("Shared %02d", index%aliasGroups), fmt.Sprintf("Topic %05d", index)},
			TargetID: fmt.Sprintf("doc-%05d", index),
		})
	}
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		ns := buildLinkNamespace(inv)
		if len(ns.notesByName) == 0 {
			b.Fatal("empty namespace")
		}
	}
}

func BenchmarkBuildLinkNamespaceCollision(b *testing.B) { benchmarkNamespace(b, 5000, 50) }
func BenchmarkBuildLinkNamespaceOrdinary(b *testing.B)  { benchmarkNamespace(b, 5000, 5000) }
