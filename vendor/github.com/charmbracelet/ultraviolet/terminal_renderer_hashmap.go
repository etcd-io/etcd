package uv

import "hash/maphash"

// hash returns the hash value of a [Line].
func hash(h *maphash.Hash, l Line) uint64 {
	h.Reset()
	for _, c := range l {
		// maphash writes can not fail
		_, _ = h.WriteString(c.Content)
	}

	return h.Sum64()
}

// hashmap represents a single [Line] hash.
type hashmap struct {
	value              uint64
	oldcount, newcount int
	oldindex, newindex int
}

// The value used to indicate lines created by insertions and scrolls.
const newIndex = -1

// updateHashmap updates the hashmap with the new hash value.
func (s *TerminalRenderer) updateHashmap(newbuf *Buffer) {
	height := newbuf.Height()

	if len(s.oldhash) >= height && len(s.newhash) >= height {
		// rehash changed lines
		for i := range height {
			if newbuf.Touched == nil || newbuf.Touched[i] != nil {
				// TODO: Investigate why this is needed. If we remove this
				// line, scroll optimization does not work correctly. This
				// should happen else where.
				s.oldhash[i] = hash(&s.hasher, s.curbuf.Line(i))
				s.newhash[i] = hash(&s.hasher, newbuf.Line(i))
			}
		}
	} else {
		// rehash all
		if len(s.oldhash) != height {
			s.oldhash = make([]uint64, height)
		}
		if len(s.newhash) != height {
			s.newhash = make([]uint64, height)
		}
		for i := range height {
			s.oldhash[i] = hash(&s.hasher, s.curbuf.Line(i))
			s.newhash[i] = hash(&s.hasher, newbuf.Line(i))
		}
	}

	s.hashtab = make([]hashmap, (height+1)*2)
	for i := range height {
		hashval := s.oldhash[i]

		// Find matching hash or empty slot
		var idx int
		for idx = 0; idx < len(s.hashtab) && s.hashtab[idx].value != 0; idx++ {
			if s.hashtab[idx].value == hashval {
				break
			}
		}

		s.hashtab[idx].value = hashval // in case this is a new hash
		s.hashtab[idx].oldcount++
		s.hashtab[idx].oldindex = i
	}
	for i := range height {
		hashval := s.newhash[i]

		// Find matching hash or empty slot
		var idx int
		for idx = 0; idx < len(s.hashtab) && s.hashtab[idx].value != 0; idx++ {
			if s.hashtab[idx].value == hashval {
				break
			}
		}

		s.hashtab[idx].value = hashval // in case this is a new hash
		s.hashtab[idx].newcount++
		s.hashtab[idx].newindex = i
		s.oldnum[i] = newIndex // init old indices slice
	}

	// Mark line pair corresponding to unique hash pairs.
	for i := 0; i < len(s.hashtab) && s.hashtab[i].value != 0; i++ {
		hsp := &s.hashtab[i]
		if hsp.oldcount == 1 && hsp.newcount == 1 && hsp.oldindex != hsp.newindex {
			s.oldnum[hsp.newindex] = hsp.oldindex
		}
	}

	s.growHunks(newbuf)

	// Eliminate bad or impossible shifts. This includes removing those hunks
	// which could not grow because of conflicts, as well those which are to be
	// moved too far, they are likely to destroy more than carry.
	for i := 0; i < height; {
		var start, shift, size int
		for i < height && s.oldnum[i] == newIndex {
			i++
		}
		if i >= height {
			break
		}
		start = i
		shift = s.oldnum[i] - i
		i++
		for i < height && s.oldnum[i] != newIndex && s.oldnum[i]-i == shift {
			i++
		}
		size = i - start
		if size < 3 || size+min(size/8, 2) < abs(shift) {
			for start < i {
				s.oldnum[start] = newIndex
				start++
			}
		}
	}

	// After clearing invalid hunks, try grow the rest.
	s.growHunks(newbuf)
}

// scrollOldhash scrolls the oldhash slice by 'n' lines between 'top' and 'bot'.
func (s *TerminalRenderer) scrollOldhash(n, top, bot int) {
	if len(s.oldhash) == 0 {
		return
	}

	size := bot - top + 1 - abs(n)
	if n > 0 {
		// Move existing hashes up
		copy(s.oldhash[top:], s.oldhash[top+n:top+n+size])
		// Recalculate hashes for newly shifted-in lines
		for i := bot; i > bot-n; i-- {
			s.oldhash[i] = hash(&s.hasher, s.curbuf.Line(i))
		}
	} else {
		// Move existing hashes down
		copy(s.oldhash[top-n:], s.oldhash[top:top+size])
		// Recalculate hashes for newly shifted-in lines
		for i := top; i < top-n; i++ {
			s.oldhash[i] = hash(&s.hasher, s.curbuf.Line(i))
		}
	}
}

func (s *TerminalRenderer) growHunks(newbuf *Buffer) {
	var (
		backLimit    int // limits for cells to fill
		backRefLimit int // limit for references
		i            int
		nextHunk     int
	)

	height := newbuf.Height()
	for i < height && s.oldnum[i] == newIndex {
		i++
	}
	for ; i < height; i = nextHunk {
		var (
			forwardLimit    int
			forwardRefLimit int
			end             int
			start           = i
			shift           = s.oldnum[i] - i
		)

		// get forward limit
		i = start + 1
		for i < height &&
			s.oldnum[i] != newIndex &&
			s.oldnum[i]-i == shift {
			i++
		}

		end = i
		for i < height && s.oldnum[i] == newIndex {
			i++
		}

		nextHunk = i
		forwardLimit = i
		if i >= height || s.oldnum[i] >= i {
			forwardRefLimit = i
		} else {
			forwardRefLimit = s.oldnum[i]
		}

		i = start - 1

		// grow back
		if shift < 0 {
			backLimit = backRefLimit + (-shift)
		}
		for i >= backLimit {
			if s.newhash[i] == s.oldhash[i+shift] ||
				s.costEffective(newbuf, i+shift, i, shift < 0) {
				s.oldnum[i] = i + shift
			} else {
				break
			}
			i--
		}

		i = end
		// grow forward
		if shift > 0 {
			forwardLimit = forwardRefLimit - shift
		}
		for i < forwardLimit {
			if s.newhash[i] == s.oldhash[i+shift] ||
				s.costEffective(newbuf, i+shift, i, shift > 0) {
				s.oldnum[i] = i + shift
			} else {
				break
			}
			i++
		}

		backLimit = i
		backRefLimit = backLimit
		if shift > 0 {
			backRefLimit += shift
		}
	}
}

// costEffective returns true if the cost of moving line 'from' to line 'to' seems to be
// cost effective. 'blank' indicates whether the line 'to' would become blank.
func (s *TerminalRenderer) costEffective(newbuf *Buffer, from, to int, blank bool) bool {
	if from == to {
		return false
	}

	newFrom := s.oldnum[from]
	if newFrom == newIndex {
		newFrom = from
	}

	// On the left side of >= is the cost before moving. On the right side --
	// cost after moving.

	// Calculate costs before moving.
	var costBeforeMove int
	if blank {
		// Cost of updating blank line at destination.
		costBeforeMove = s.updateCostBlank(newbuf, newbuf.Line(to))
	} else {
		// Cost of updating exiting line at destination.
		costBeforeMove = s.updateCost(newbuf, s.curbuf.Line(to), newbuf.Line(to))
	}

	// Add cost of updating source line
	costBeforeMove += s.updateCost(newbuf, s.curbuf.Line(newFrom), newbuf.Line(from))

	// Calculate costs after moving.
	var costAfterMove int
	if newFrom == from {
		// Source becomes blank after move
		costAfterMove = s.updateCostBlank(newbuf, newbuf.Line(from))
	} else {
		// Source gets updated from another line
		costAfterMove = s.updateCost(newbuf, s.curbuf.Line(newFrom), newbuf.Line(from))
	}

	// Add cost of moving source line to destination
	costAfterMove += s.updateCost(newbuf, s.curbuf.Line(from), newbuf.Line(to))

	// Return true if moving is cost effective (costs less or equal)
	return costBeforeMove >= costAfterMove
}

func (s *TerminalRenderer) updateCost(newbuf *Buffer, from, to Line) (cost int) {
	var fidx, tidx int
	for i := newbuf.Width() - 1; i > 0; i, fidx, tidx = i-1, fidx+1, tidx+1 {
		if !cellEqual(from.At(fidx), to.At(tidx)) {
			cost++
		}
	}
	return
}

func (s *TerminalRenderer) updateCostBlank(newbuf *Buffer, to Line) (cost int) {
	// This assumes bce capability.
	blank := s.clearBlank()
	var tidx int
	for i := newbuf.Width() - 1; i > 0; i, tidx = i-1, tidx+1 {
		if !cellEqual(blank, to.At(tidx)) {
			cost++
		}
	}
	return
}
