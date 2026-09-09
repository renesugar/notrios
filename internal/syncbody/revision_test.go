package syncbody

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func revision(id string, parents ...string) Revision {
	return Revision{
		ID: id, DocumentID: "doc_one", Parents: NormalizeParents(parents),
		ContentSHA256: strings.Repeat("0", 63) + "1", ContentLength: 1,
		AuthorReplicaID: "replica-a", AuthorSequence: 1,
	}
}

func mustGraph(t *testing.T, revisions ...Revision) Graph {
	t.Helper()
	graph, err := NewGraph(revisions)
	if err != nil {
		t.Fatalf("NewGraph: %v", err)
	}
	return graph
}

func TestHeadsAndAncestry(t *testing.T) {
	t.Parallel()
	graph := mustGraph(t,
		revision("r0"),
		revision("r1", "r0"),
		revision("ra", "r1"),
		revision("rb", "r1"),
	)
	heads := graph.Heads()
	if len(heads) != 2 || heads[0] != "ra" || heads[1] != "rb" {
		t.Fatalf("heads = %v, want [ra rb]", heads)
	}
	if !graph.IsAncestor("r0", "ra") || !graph.IsAncestor("r1", "rb") {
		t.Fatal("ancestry through the chain was not found")
	}
	if !graph.IsAncestor("ra", "ra") {
		t.Fatal("a revision must be its own ancestor")
	}
	if graph.IsAncestor("ra", "rb") || graph.IsAncestor("rb", "ra") {
		t.Fatal("two heads must not be ancestors of each other")
	}

	linear := mustGraph(t, revision("r0"), revision("r1", "r0"))
	if heads := linear.Heads(); len(heads) != 1 || heads[0] != "r1" {
		t.Fatalf("a linear history has one head, got %v", heads)
	}
}

func TestMergeBase(t *testing.T) {
	t.Parallel()
	t.Run("simple-fork", func(t *testing.T) {
		graph := mustGraph(t,
			revision("r0"), revision("r1", "r0"),
			revision("ra", "r1"), revision("rb", "r1"),
		)
		base, err := graph.MergeBase("ra", "rb")
		if err != nil || base != "r1" {
			t.Fatalf("MergeBase = %q, %v; want r1", base, err)
		}
	})

	t.Run("uneven-depth", func(t *testing.T) {
		graph := mustGraph(t,
			revision("r0"), revision("r1", "r0"),
			revision("ra1", "r1"), revision("ra2", "ra1"), revision("ra3", "ra2"),
			revision("rb1", "r1"),
		)
		base, err := graph.MergeBase("ra3", "rb1")
		if err != nil || base != "r1" {
			t.Fatalf("MergeBase = %q, %v; want r1", base, err)
		}
	})

	t.Run("one-side-already-contains-the-other", func(t *testing.T) {
		graph := mustGraph(t, revision("r0"), revision("r1", "r0"), revision("r2", "r1"))
		base, err := graph.MergeBase("r2", "r0")
		if err != nil || base != "r0" {
			t.Fatalf("MergeBase = %q, %v; want r0", base, err)
		}
	})

	t.Run("after-an-earlier-merge", func(t *testing.T) {
		graph := mustGraph(t,
			revision("r0"),
			revision("ra", "r0"), revision("rb", "r0"),
			revision("rm", "ra", "rb"),
			revision("rc", "rm"), revision("rd", "rm"),
		)
		base, err := graph.MergeBase("rc", "rd")
		if err != nil || base != "rm" {
			t.Fatalf("MergeBase = %q, %v; want rm", base, err)
		}
	})

	t.Run("order-independent", func(t *testing.T) {
		graph := mustGraph(t,
			revision("r0"), revision("ra", "r0"), revision("rb", "r0"),
		)
		forward, _ := graph.MergeBase("ra", "rb")
		reverse, _ := graph.MergeBase("rb", "ra")
		if forward != reverse {
			t.Fatalf("MergeBase is order dependent: %q vs %q", forward, reverse)
		}
	})

	t.Run("unknown-revision-refuses", func(t *testing.T) {
		graph := mustGraph(t, revision("r0"))
		if _, err := graph.MergeBase("r0", "missing"); !errors.Is(err, ErrUnknownRevision) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("unrelated-roots-refuse", func(t *testing.T) {
		graph := mustGraph(t, revision("r0"), revision("s0"))
		if _, err := graph.MergeBase("r0", "s0"); !errors.Is(err, ErrUnknownRevision) {
			t.Fatal("two unrelated roots must not produce a base")
		}
	})

	t.Run("divergence-bound-refuses-rather-than-guessing", func(t *testing.T) {
		revisions := []Revision{revision("r0")}
		previous := "r0"
		for index := 0; index < MaxAncestorNodes+10; index++ {
			id := fmt.Sprintf("a%06d", index)
			revisions = append(revisions, revision(id, previous))
			previous = id
		}
		revisions = append(revisions, revision("b1", "r0"))
		graph := mustGraph(t, revisions...)
		if _, err := graph.MergeBase(previous, "b1"); !errors.Is(err, ErrGraphTooLarge) {
			t.Fatalf("err = %v, want a bound refusal", err)
		}
	})

	t.Run("long-shared-history-costs-only-the-divergence", func(t *testing.T) {
		// The same chain length is fine when the two heads parted recently,
		// which is the property that makes the bound about divergence.
		revisions := []Revision{revision("r0")}
		previous := "r0"
		for index := 0; index < MaxAncestorNodes+10; index++ {
			id := fmt.Sprintf("c%06d", index)
			revisions = append(revisions, revision(id, previous))
			previous = id
		}
		revisions = append(revisions, revision("x1", previous), revision("y1", previous))
		graph := mustGraph(t, revisions...)
		base, err := graph.MergeBase("x1", "y1")
		if err != nil || base != previous {
			t.Fatalf("MergeBase = %q, %v; want %q", base, err, previous)
		}
	})
}

func TestGraphRefusesAnIncompleteHistory(t *testing.T) {
	t.Parallel()
	if _, err := NewGraph([]Revision{revision("r1", "missing")}); !errors.Is(err, ErrUnknownRevision) {
		t.Fatalf("a parent that is not present must be refused, got %v", err)
	}
	if _, err := NewGraph([]Revision{revision("r0"), revision("r0")}); !errors.Is(err, ErrUnknownRevision) {
		t.Fatalf("duplicate revisions must be refused, got %v", err)
	}
	tooManyParents := revision("r1")
	for index := 0; index <= MaxRevisionParents; index++ {
		tooManyParents.Parents = append(tooManyParents.Parents, fmt.Sprintf("p%d", index))
	}
	if _, err := NewGraph([]Revision{tooManyParents}); !errors.Is(err, ErrGraphTooLarge) {
		t.Fatalf("parent-count bound: %v", err)
	}
}

func TestDerivedIdentitiesAreReplicaIndependent(t *testing.T) {
	t.Parallel()
	const content = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	// The same merge computed on two replicas must produce one revision, so
	// the second copy to arrive is an exact replay rather than a rival head.
	left := MergeRevisionID("doc_one", []string{"ra", "rb"}, content)
	right := MergeRevisionID("doc_one", []string{"rb", "ra"}, content)
	if left != right {
		t.Fatalf("parent order changed the merge id: %q vs %q", left, right)
	}
	if !strings.HasPrefix(left, "rev_") || len(left) != len("rev_")+26 {
		t.Fatalf("a merge revision id must be shaped like any other revision id: %q", left)
	}
	if MergeRevisionID("doc_one", []string{"ra", "rb"}, strings.Repeat("a", 64)) == left {
		t.Fatal("different merged content produced the same merge id")
	}
	if MergeRevisionID("doc_two", []string{"ra", "rb"}, content) == left {
		t.Fatal("a different document produced the same merge id")
	}
	if MergeRevisionID("doc_one", []string{"ra", "rc"}, content) == left {
		t.Fatal("different parents produced the same merge id")
	}

	conflict := ConflictID("doc_one", "r1", []string{"ra", "rb"})
	if conflict != ConflictID("doc_one", "r1", []string{"rb", "ra"}) {
		t.Fatal("a conflict must have one identity whichever side is called local")
	}
	if !strings.HasPrefix(conflict, "cfl_") {
		t.Fatalf("conflict id %q", conflict)
	}
	if ConflictID("doc_one", "r0", []string{"ra", "rb"}) == conflict {
		t.Fatal("a different merge base produced the same conflict id")
	}

	// The two derivations must not collide even on identical inputs, because
	// one names content and the other names a disagreement about it.
	if strings.TrimPrefix(conflict, "cfl_") == strings.TrimPrefix(MergeRevisionID("doc_one", []string{"ra", "rb"}, "r1"), "rev_") {
		t.Fatal("conflict and merge identities share a domain")
	}
}

func TestNormalizeParents(t *testing.T) {
	t.Parallel()
	got := NormalizeParents([]string{"rb", "", "ra", "rb"})
	if len(got) != 2 || got[0] != "ra" || got[1] != "rb" {
		t.Fatalf("NormalizeParents = %v", got)
	}
	if NormalizeParents(nil) == nil {
		t.Fatal("NormalizeParents must return an empty slice rather than nil")
	}
}
