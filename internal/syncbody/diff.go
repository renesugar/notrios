package syncbody

// change is one replaced token range: base[BaseStart:BaseEnd) became
// other[OtherStart:OtherEnd). An insertion has an empty base range and a
// deletion an empty other range.
type change struct {
	BaseStart, BaseEnd   int
	OtherStart, OtherEnd int
}

func (c change) baseLen() int  { return c.BaseEnd - c.BaseStart }
func (c change) otherLen() int { return c.OtherEnd - c.OtherStart }

// intern maps token strings onto small integers so the diff compares by value
// rather than by string, and so a pathological body of long identical lines
// costs one comparison per token instead of one per byte.
func intern(sequences ...[]string) [][]int32 {
	table := make(map[string]int32)
	interned := make([][]int32, len(sequences))
	for index, sequence := range sequences {
		values := make([]int32, len(sequence))
		for position, token := range sequence {
			id, seen := table[token]
			if !seen {
				id = int32(len(table))
				table[token] = id
			}
			values[position] = id
		}
		interned[index] = values
	}
	return interned
}

// diffChanges returns the minimal edit script from base to other using Myers'
// O(ND) algorithm, bounded by maxDistance. It returns ok=false rather than a
// partial answer when the two sequences differ by more edits than the bound
// allows: a caller must be able to tell "these are too far apart to merge" from
// "these merge cleanly", and a truncated edit script says neither.
func diffChanges(base, other []int32, maxDistance int) ([]change, bool) {
	// Trimming the common ends first is what keeps the bound realistic. A
	// localized edit in a large note has an edit distance of a few tokens once
	// the shared prefix and suffix are removed, whatever the note's size.
	prefix := 0
	for prefix < len(base) && prefix < len(other) && base[prefix] == other[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(base)-prefix && suffix < len(other)-prefix &&
		base[len(base)-1-suffix] == other[len(other)-1-suffix] {
		suffix++
	}
	trimmedBase := base[prefix : len(base)-suffix]
	trimmedOther := other[prefix : len(other)-suffix]
	switch {
	case len(trimmedBase) == 0 && len(trimmedOther) == 0:
		return nil, true
	case len(trimmedBase) == 0:
		return []change{{prefix, prefix, prefix, prefix + len(trimmedOther)}}, true
	case len(trimmedOther) == 0:
		return []change{{prefix, prefix + len(trimmedBase), prefix, prefix}}, true
	}

	matches, ok := myersMatches(trimmedBase, trimmedOther, maxDistance)
	if !ok {
		return nil, false
	}
	changes := make([]change, 0, len(matches)+1)
	baseCursor, otherCursor := 0, 0
	for _, match := range matches {
		if match[0] > baseCursor || match[1] > otherCursor {
			changes = append(changes, change{
				prefix + baseCursor, prefix + match[0],
				prefix + otherCursor, prefix + match[1],
			})
		}
		baseCursor, otherCursor = match[0]+1, match[1]+1
	}
	if baseCursor < len(trimmedBase) || otherCursor < len(trimmedOther) {
		changes = append(changes, change{
			prefix + baseCursor, prefix + len(trimmedBase),
			prefix + otherCursor, prefix + len(trimmedOther),
		})
	}
	return changes, true
}

// myersMatches returns the matched (base, other) index pairs of one shortest
// edit script, ascending. Each trace entry is the furthest-reaching frontier
// *before* its edit step, which is what the backtrack reads; snapshots are
// trimmed to the diagonals reachable at that depth, so the whole trace costs
// (maxDistance+1)² integers rather than a full frontier per step.
func myersMatches(base, other []int32, maxDistance int) ([][2]int, bool) {
	n, m := len(base), len(other)
	bound := n + m
	if maxDistance < bound {
		bound = maxDistance
	}
	offset := bound + 2
	frontier := make([]int, 2*bound+5)
	trace := make([][]int, 0, bound+1)
	for distance := 0; distance <= bound; distance++ {
		snapshot := make([]int, 2*distance+1)
		copy(snapshot, frontier[offset-distance:offset+distance+1])
		trace = append(trace, snapshot)
		for diagonal := -distance; diagonal <= distance; diagonal += 2 {
			var x int
			switch {
			case diagonal == -distance:
				x = frontier[offset+diagonal+1]
			case diagonal != distance && frontier[offset+diagonal-1] < frontier[offset+diagonal+1]:
				x = frontier[offset+diagonal+1]
			default:
				x = frontier[offset+diagonal-1] + 1
			}
			y := x - diagonal
			for x < n && y < m && base[x] == other[y] {
				x++
				y++
			}
			frontier[offset+diagonal] = x
			if x >= n && y >= m {
				return backtrack(trace, n, m), true
			}
		}
	}
	return nil, false
}

func backtrack(trace [][]int, n, m int) [][2]int {
	matches := make([][2]int, 0, n)
	x, y := n, m
	for distance := len(trace) - 1; distance >= 0; distance-- {
		snapshot := trace[distance]
		at := func(diagonal int) int {
			index := diagonal + distance
			if index < 0 || index >= len(snapshot) {
				return -1
			}
			return snapshot[index]
		}
		diagonal := x - y
		var previousDiagonal int
		if diagonal == -distance || (diagonal != distance && at(diagonal-1) < at(diagonal+1)) {
			previousDiagonal = diagonal + 1
		} else {
			previousDiagonal = diagonal - 1
		}
		previousX, previousY := at(previousDiagonal), 0
		if distance == 0 {
			previousX, previousY = 0, 0
		} else {
			previousY = previousX - previousDiagonal
		}
		for x > previousX && y > previousY {
			x--
			y--
			matches = append(matches, [2]int{x, y})
		}
		x, y = previousX, previousY
	}
	for left, right := 0, len(matches)-1; left < right; left, right = left+1, right-1 {
		matches[left], matches[right] = matches[right], matches[left]
	}
	return matches
}
