package syncbody

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	// MaxRevisionParents bounds one revision's parent list. An ordinary
	// revision has one parent and a merge has two; the headroom exists so a
	// future octopus merge is a decision rather than a schema change.
	MaxRevisionParents = 8

	// MaxAncestorNodes bounds a merge-base search. Because the search expands
	// both heads one generation at a time and stops at the first common
	// revision, it is a bound on how far two replicas have *diverged*, not on
	// how long the note's history is. A note edited ten thousand times still
	// merges cheaply if the two replicas parted company recently.
	MaxAncestorNodes = 10_000
)

var (
	// ErrUnknownRevision reports a parent naming a revision this replica does
	// not hold. It is a refusal: an incomplete graph cannot be merged, and
	// guessing an ancestor would silently discard whatever sits between.
	ErrUnknownRevision = errors.New("revision graph names an unknown revision")
	// ErrGraphTooLarge reports that a bound stopped the search.
	ErrGraphTooLarge = errors.New("revision graph exceeds its bound")
)

// Revision is one immutable note revision object. Content is named by exact
// hash and length rather than carried, so the graph can be reasoned about
// before any body is fetched or reconstructed.
type Revision struct {
	ID              string
	DocumentID      string
	Parents         []string
	ContentSHA256   string
	ContentLength   int
	AuthorReplicaID string
	AuthorSequence  int64
}

// Graph is a bounded read model of one document's revisions.
type Graph struct {
	byID     map[string]Revision
	children map[string][]string
}

// NewGraph indexes one document's revisions. A parent naming a revision that is
// not present is an error rather than a silently pruned edge.
func NewGraph(revisions []Revision) (Graph, error) {
	graph := Graph{
		byID:     make(map[string]Revision, len(revisions)),
		children: make(map[string][]string, len(revisions)),
	}
	for _, revision := range revisions {
		if revision.ID == "" {
			return Graph{}, fmt.Errorf("%w: empty revision id", ErrUnknownRevision)
		}
		if len(revision.Parents) > MaxRevisionParents {
			return Graph{}, fmt.Errorf("%w: %s has %d parents", ErrGraphTooLarge, revision.ID, len(revision.Parents))
		}
		if _, duplicate := graph.byID[revision.ID]; duplicate {
			return Graph{}, fmt.Errorf("%w: duplicate revision %s", ErrUnknownRevision, revision.ID)
		}
		graph.byID[revision.ID] = revision
	}
	for _, revision := range revisions {
		for _, parent := range revision.Parents {
			if _, known := graph.byID[parent]; !known {
				return Graph{}, fmt.Errorf("%w: %s names parent %s", ErrUnknownRevision, revision.ID, parent)
			}
			graph.children[parent] = append(graph.children[parent], revision.ID)
		}
	}
	return graph, nil
}

// Get returns one revision.
func (g Graph) Get(id string) (Revision, bool) {
	revision, found := g.byID[id]
	return revision, found
}

// Len returns the number of revisions in the graph.
func (g Graph) Len() int { return len(g.byID) }

// Heads returns the revisions no other revision claims as a parent, sorted by
// ID. More than one head means the document has diverged and needs a merge.
func (g Graph) Heads() []string {
	heads := make([]string, 0, 2)
	for id := range g.byID {
		if len(g.children[id]) == 0 {
			heads = append(heads, id)
		}
	}
	sort.Strings(heads)
	return heads
}

// IsAncestor reports whether ancestor is reachable from descendant through
// parent edges. A revision is its own ancestor, which is what makes
// "already contains" and "identical" one question rather than two.
func (g Graph) IsAncestor(ancestor, descendant string) bool {
	if ancestor == descendant {
		return true
	}
	visited := map[string]bool{descendant: true}
	frontier := []string{descendant}
	for len(frontier) > 0 && len(visited) <= MaxAncestorNodes {
		current := frontier[0]
		frontier = frontier[1:]
		for _, parent := range g.byID[current].Parents {
			if parent == ancestor {
				return true
			}
			if !visited[parent] {
				visited[parent] = true
				frontier = append(frontier, parent)
			}
		}
	}
	return false
}

// MergeBase returns the revision to use as the base of a three-way merge of
// left and right: the nearest revision both descend from. The search expands
// each side one generation at a time and stops as soon as the two reachable
// sets meet, so its cost tracks divergence rather than history.
//
// When several equally near candidates exist, the one with the lowest ID wins.
// The tie-break has to be total and content independent, because two replicas
// computing this separately must reach the same base or their "identical"
// merges would not be identical at all.
func (g Graph) MergeBase(left, right string) (string, error) {
	if _, found := g.byID[left]; !found {
		return "", fmt.Errorf("%w: %s", ErrUnknownRevision, left)
	}
	if _, found := g.byID[right]; !found {
		return "", fmt.Errorf("%w: %s", ErrUnknownRevision, right)
	}
	if left == right {
		return left, nil
	}
	seenLeft := map[string]bool{left: true}
	seenRight := map[string]bool{right: true}
	frontierLeft := []string{left}
	frontierRight := []string{right}
	visited := 2
	var candidates []string
	for len(candidates) == 0 && (len(frontierLeft) > 0 || len(frontierRight) > 0) {
		if visited > MaxAncestorNodes {
			return "", fmt.Errorf("%w: merge base search visited more than %d revisions", ErrGraphTooLarge, MaxAncestorNodes)
		}
		frontierLeft, candidates = g.expand(frontierLeft, seenLeft, seenRight, candidates, &visited)
		frontierRight, candidates = g.expand(frontierRight, seenRight, seenLeft, candidates, &visited)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: %s and %s share no ancestor", ErrUnknownRevision, left, right)
	}
	// Drop any candidate that another candidate already descends from: it is a
	// more distant ancestor and would throw away edits both sides kept.
	best := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		superseded := false
		for _, other := range candidates {
			if other != candidate && g.IsAncestor(candidate, other) {
				superseded = true
				break
			}
		}
		if !superseded {
			best = append(best, candidate)
		}
	}
	sort.Strings(best)
	return best[0], nil
}

func (g Graph) expand(frontier []string, seen, otherSeen map[string]bool, candidates []string, visited *int) ([]string, []string) {
	next := make([]string, 0, len(frontier))
	for _, current := range frontier {
		parents := append([]string(nil), g.byID[current].Parents...)
		sort.Strings(parents)
		for _, parent := range parents {
			if seen[parent] {
				continue
			}
			seen[parent] = true
			*visited++
			if otherSeen[parent] {
				candidates = append(candidates, parent)
				continue
			}
			next = append(next, parent)
		}
	}
	return next, candidates
}

// NormalizeParents returns a sorted, deduplicated parent list. Parent order
// carries no meaning in Notrios' revision graph, and leaving it free would let
// two replicas derive different IDs for the same merge.
func NormalizeParents(parents []string) []string {
	unique := make([]string, 0, len(parents))
	seen := make(map[string]bool, len(parents))
	for _, parent := range parents {
		if parent == "" || seen[parent] {
			continue
		}
		seen[parent] = true
		unique = append(unique, parent)
	}
	sort.Strings(unique)
	return unique
}

// MergeRevisionID derives a merge revision's identity from the document, its
// normalized parents, and the exact merged content. Two replicas that compute
// the same merge therefore produce the same revision rather than two revisions
// holding identical bytes, and the second one to arrive is an exact replay that
// admission already knows how to discard.
//
// The shape matches store.NewID output, because a merge revision is an
// ordinary revision and nothing downstream should be able to tell them apart.
func MergeRevisionID(documentID string, parents []string, contentSHA256 string) string {
	return derivedID("rev", "notrios.revision.merge.v1", append([]string{documentID, contentSHA256}, NormalizeParents(parents)...))
}

// ConflictID derives a conflict's identity from the document, the merge base,
// and the two conflicting revisions in sorted order. Sorting is what makes the
// identity replica independent: "mine versus theirs" is a different pair on
// each side, and a conflict that had two identities would be reported twice.
func ConflictID(documentID, baseRevisionID string, revisions []string) string {
	return derivedID("cfl", "notrios.conflict.v1", append([]string{documentID, baseRevisionID}, NormalizeParents(revisions)...))
}

func derivedID(prefix, domain string, parts []string) string {
	digest := sha256.New()
	digest.Write([]byte(domain))
	for _, part := range parts {
		digest.Write([]byte{0})
		digest.Write([]byte(part))
	}
	sum := digest.Sum(nil)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:16])
	return strings.ToLower(prefix + "_" + encoded)
}
