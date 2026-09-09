package syncbody

// conflictSpan is one region where both sides changed the same base tokens and
// disagreed. It keeps all three token slices, because a conflict that discards
// the base has thrown away the only evidence of what each side did.
type conflictSpan struct {
	baseStart, localStart, remoteStart int
	base, local, remote                []string
}

// segment is either resolved output or an unresolved span, in output order.
type segment struct {
	resolved []string
	conflict *conflictSpan
}

// merge3 is the granularity-independent three-way merge. It runs over lines for
// a whole body and over words for one conflicting region; nothing in it knows
// which. It returns ok=false when either diff exceeded its edit-distance bound,
// since a truncated edit script would silently drop text.
func merge3(base, local, remote []string, maxDistance int) ([]segment, bool) {
	interned := intern(base, local, remote)
	localChanges, ok := diffChanges(interned[0], interned[1], maxDistance)
	if !ok {
		return nil, false
	}
	remoteChanges, ok := diffChanges(interned[0], interned[2], maxDistance)
	if !ok {
		return nil, false
	}

	segments := make([]segment, 0, len(localChanges)+len(remoteChanges)+1)
	baseCursor, localCursor, remoteCursor := 0, 0, 0
	localIndex, remoteIndex := 0, 0
	for {
		next := -1
		if localIndex < len(localChanges) {
			next = localChanges[localIndex].BaseStart
		}
		if remoteIndex < len(remoteChanges) && (next < 0 || remoteChanges[remoteIndex].BaseStart < next) {
			next = remoteChanges[remoteIndex].BaseStart
		}
		if next < 0 {
			segments = append(segments, segment{resolved: base[baseCursor:]})
			return segments, true
		}
		if next > baseCursor {
			segments = append(segments, segment{resolved: base[baseCursor:next]})
			stable := next - baseCursor
			baseCursor, localCursor, remoteCursor = next, localCursor+stable, remoteCursor+stable
		}

		// Grow the region until neither side has a change overlapping it. Two
		// pure insertions at the same base position overlap each other; an
		// insertion merely adjacent to a replacement does not, which is what
		// keeps genuinely disjoint neighbouring edits out of a false conflict.
		regionEnd := baseCursor
		localEnd, remoteEnd := localCursor, remoteCursor
		localTaken, remoteTaken := 0, 0
		for {
			grew := false
			for localIndex+localTaken < len(localChanges) {
				candidate := localChanges[localIndex+localTaken]
				if !overlapsRegion(candidate, baseCursor, regionEnd) {
					break
				}
				localEnd = candidate.OtherEnd
				if candidate.BaseEnd > regionEnd {
					regionEnd = candidate.BaseEnd
				}
				localTaken++
				grew = true
			}
			for remoteIndex+remoteTaken < len(remoteChanges) {
				candidate := remoteChanges[remoteIndex+remoteTaken]
				if !overlapsRegion(candidate, baseCursor, regionEnd) {
					break
				}
				remoteEnd = candidate.OtherEnd
				if candidate.BaseEnd > regionEnd {
					regionEnd = candidate.BaseEnd
				}
				remoteTaken++
				grew = true
			}
			if !grew {
				break
			}
		}
		// The region can extend past the last change one side made — the other
		// side grew it. Those trailing base tokens are untouched on this side,
		// so they map one for one.
		localEnd += regionEnd - lastBaseEnd(localChanges, localIndex, localTaken, baseCursor)
		remoteEnd += regionEnd - lastBaseEnd(remoteChanges, remoteIndex, remoteTaken, baseCursor)

		baseSlice := base[baseCursor:regionEnd]
		localSlice := local[localCursor:localEnd]
		remoteSlice := remote[remoteCursor:remoteEnd]
		switch {
		case remoteTaken == 0:
			segments = append(segments, segment{resolved: localSlice})
		case localTaken == 0:
			segments = append(segments, segment{resolved: remoteSlice})
		case equalTokens(localSlice, remoteSlice):
			// Both replicas made the same change. This is convergence, not a
			// conflict, and it is common whenever the same fix is applied twice.
			segments = append(segments, segment{resolved: localSlice})
		default:
			span := conflictSpan{
				baseStart: baseCursor, localStart: localCursor, remoteStart: remoteCursor,
				base: baseSlice, local: localSlice, remote: remoteSlice,
			}
			segments = append(segments, segment{conflict: &span})
		}
		baseCursor, localCursor, remoteCursor = regionEnd, localEnd, remoteEnd
		localIndex += localTaken
		remoteIndex += remoteTaken
	}
}

// overlapsRegion reports whether a change belongs to the region [start, end).
// An empty region (a pure insertion point) is overlapped only by a change that
// begins exactly there.
func overlapsRegion(candidate change, start, end int) bool {
	if end == start {
		return candidate.BaseStart == start
	}
	return candidate.BaseStart < end
}

// lastBaseEnd returns the base index the final consumed change reached, which
// is where one-for-one mapping resumes for the rest of the region.
func lastBaseEnd(changes []change, index, taken, fallback int) int {
	if taken == 0 {
		return fallback
	}
	return changes[index+taken-1].BaseEnd
}
