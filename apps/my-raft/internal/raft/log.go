package raft

type unstable struct {
	// Entries that have not been persisted yet.
	entries []Entry
	// Index of the first entry that has not been persisted yet.
	offset uint64
}

func (u *unstable) mustCheckOutOfBounds(l, r uint64) {
	if l > r {
		panic("invalid unstable slice")
	}

	if l < u.offset || r > u.offset+uint64(len(u.entries)) {
		panic("invalid unstable slice")
	}
}

func (u *unstable) slice(l, r uint64) []Entry {
	u.mustCheckOutOfBounds(l, r)
	return u.entries[l-u.offset : r-u.offset]
}

// truncateAndAppend appends entries to the unstable region; this may truncate
// or overwrite previously appended data.
func (u *unstable) truncateAndAppend(ents []Entry) {
	after := ents[0].Index
	switch {
	case after == u.offset+uint64(len(u.entries)):
		u.entries = append(u.entries, ents...)
	case after <= u.offset:
		u.offset = after
		u.entries = ents
	default:
		u.entries = append([]Entry{}, u.slice(u.offset, after)...)
		u.entries = append(u.entries, ents...)
	}
}

func (u *unstable) maybeLastIndex() (uint64, bool) {
	if l := len(u.entries); l != 0 {
		return u.offset + uint64(l) - 1, true
	}
	return 0, false
}

func (u *unstable) maybeTerm(i uint64) (uint64, bool) {
	if i < u.offset {
		return 0, false
	}

	last, ok := u.maybeLastIndex()
	if !ok {
		return 0, false
	}

	if i > last {
		return 0, false
	}

	return u.entries[i-u.offset].Term, true
}

func (u *unstable) stableTo(i, t uint64) {
	gt, ok := u.maybeTerm(i)
	if !ok {
		return
	}

	if t == gt && i >= u.offset {
		u.entries = u.entries[i-u.offset+1:]
		u.offset = i + 1
	}
}

type raftLog struct {
	// Storage interface, providing lookups over persisted entries.
	storage Storage
	// Entries that have not been persisted yet.
	unstable unstable
	// Index of the highest committed entry.
	commitIndex uint64
	// Index of the highest applied entry.
	applyIndex uint64
}

func newRaftLog(storage Storage) *raftLog {
	if storage == nil {
		panic("storage must not be nil")
	}

	r := raftLog{
		storage: storage,
	}

	firstIndex, err := storage.FirstIndex()
	if err != nil {
		panic(err)
	}

	lastIndex, err := storage.LastIndex()
	if err != nil {
		panic(err)
	}

	r.unstable.offset = lastIndex + 1
	r.commitIndex = firstIndex - 1
	r.applyIndex = firstIndex - 1
	return &r
}

func (r *raftLog) stableTo(i, t uint64) {
	r.unstable.stableTo(i, t)
}

func (r *raftLog) unstableEntries() []Entry {
	return r.unstable.entries
}

// nextEnts returns the entries that are committed but not yet applied.
func (r *raftLog) nextEnts() []Entry {
	off := max(r.applyIndex+1, r.firstIndex())
	if r.commitIndex+1 > off {
		ents, err := r.slice(off, r.commitIndex+1)
		if err != nil {
			panic(err)
		}
		return ents
	}
	return nil
}

func (r *raftLog) firstIndex() uint64 {
	index, err := r.storage.FirstIndex()
	if err != nil {
		panic(err)
	}
	return index
}

func (r *raftLog) mustCheckOutOfBounds(lo, hi uint64) error {
	if lo > hi {
		panic("invalid raft log index")
	}

	fi := r.firstIndex()
	if lo < fi {
		panic("invalid raft log index")
	}

	if hi > r.lastIndex()+1 {
		panic("invalid raft log index")
	}

	return nil
}

func (r *raftLog) slice(lo, hi uint64) ([]Entry, error) {
	r.mustCheckOutOfBounds(lo, hi)
	if lo == hi {
		return nil, nil
	}

	var ents []Entry
	if lo < r.unstable.offset {
		entries, err := r.storage.Entries(lo, min(r.unstable.offset, hi))
		if err != nil {
			panic(err)
		}

		ents = append(ents, entries...)
	}

	if hi > r.unstable.offset {
		unstable := r.unstable.slice(max(lo, r.unstable.offset), hi)
		ents = append(ents, unstable...)
	}

	return ents, nil
}

// append only adds entries to the unstable (not yet persisted) region.
func (r *raftLog) append(ents ...Entry) uint64 {
	if len(ents) == 0 {
		return r.lastIndex()
	}

	if after := ents[0].Index - 1; after < r.commitIndex {
		panic("entry index less than commit index")
	}

	r.unstable.truncateAndAppend(ents)
	return r.lastIndex()
}

func (r *raftLog) lastIndex() uint64 {
	if i, ok := r.unstable.maybeLastIndex(); ok {
		return i
	}

	i, err := r.storage.LastIndex()
	if err != nil {
		panic(err)
	}

	return i
}

func (r *raftLog) lastTerm() uint64 {
	t, err := r.term(r.lastIndex())
	if err != nil {
		panic(err)
	}
	return t
}

// isUpToDate reports whether the (index, term) log is at least as up to date
// as the local log.
func (r *raftLog) isUpToDate(index, term uint64) bool {
	return term > r.lastTerm() || (term == r.lastTerm() && index >= r.lastIndex())
}

func (r *raftLog) appliedTo(i uint64) {
	if i == 0 {
		return
	}
	if r.commitIndex < i || i < r.applyIndex {
		panic("invalid apply index")
	}
	r.applyIndex = i
}

// entries returns the entries starting at index i.
func (r *raftLog) entries(i uint64) ([]Entry, error) {
	if i > r.lastIndex() {
		return nil, nil
	}
	return r.slice(i, r.lastIndex()+1)
}

func (r *raftLog) term(i uint64) (uint64, error) {
	dummyIndex := r.firstIndex() - 1
	if i < dummyIndex || i > r.lastIndex() {
		return 0, nil
	}

	// Look in the unstable region first.
	if t, ok := r.unstable.maybeTerm(i); ok {
		return t, nil
	}

	// Fall back to persisted storage.
	t, err := r.storage.Term(i)
	if err == nil {
		return t, nil
	}

	if err == ErrCompacted || err == ErrUnavailable {
		return 0, err
	}

	panic(err)
}

// commitTo updates the commit index.
func (r *raftLog) commitTo(tocommit uint64) {
	if r.commitIndex >= tocommit {
		return
	}

	if r.lastIndex() < tocommit {
		panic("commit index over last index")
	}

	r.commitIndex = tocommit
}

// maybeCommit advances the commit index to i if i is higher than the current
// commit index. It reports whether the commit index advanced.
func (r *raftLog) maybeCommit(i uint64) bool {
	if i <= r.commitIndex {
		return false
	}
	r.commitTo(i)
	return true
}

func (r *raftLog) maybeAppend(logIndex, logTerm, commitIndex uint64, ents ...Entry) (uint64, bool) {
	// Bail out if the preceding entry's index and term do not match.
	if !r.matchTerm(logIndex, logTerm) {
		return 0, false
	}

	// Find the first conflicting entry and append from there on.
	lastNewI := logIndex + uint64(len(ents))
	ci := r.findConflict(ents)
	switch {
	case ci == 0:
		// All entries already match the local log.
	case ci <= r.commitIndex:
		panic("conflict before commit index")
	default:
		offset := logIndex + 1
		r.append(ents[ci-offset:]...)
	}

	// Update the committed index.
	r.commitTo(min(commitIndex, lastNewI))
	return lastNewI, true
}

func (r *raftLog) findConflict(ents []Entry) uint64 {
	for _, ent := range ents {
		if r.matchTerm(ent.Index, ent.Term) {
			continue
		}

		return ent.Index
	}
	return 0
}

func (r *raftLog) matchTerm(logIndex, logTerm uint64) bool {
	t, err := r.term(logIndex)
	if err != nil {
		return false
	}
	return t == logTerm
}
