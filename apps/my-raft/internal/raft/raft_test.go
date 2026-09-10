package raft

import (
	"sort"
	"testing"
)

func TestIsResponseMsg(t *testing.T) {
	responses := []MessageType{MsgAppResp, MsgHeartbeatResp, MsgVoteResp, MsgPreVoteResp}
	for _, m := range responses {
		if !IsResponseMsg(m) {
			t.Errorf("IsResponseMsg(%v) = false, want true", m)
		}
	}

	nonResponses := []MessageType{MsgHup, MsgBeat, MsgProp, MsgApp, MsgVote, MsgHeartbeat, MsgPreVote, MsgReadIndex}
	for _, m := range nonResponses {
		if IsResponseMsg(m) {
			t.Errorf("IsResponseMsg(%v) = true, want false", m)
		}
	}
}

func TestNumOfPendingConf(t *testing.T) {
	ents := []Entry{
		{Type: EntryNormal},
		{Type: EntryConfChange},
		{Type: EntryNormal},
		{Type: EntryConfChange},
	}
	if got := numOfPendingConf(ents); got != 2 {
		t.Errorf("numOfPendingConf = %d, want 2", got)
	}
	if got := numOfPendingConf(nil); got != 0 {
		t.Errorf("numOfPendingConf(nil) = %d, want 0", got)
	}
}

func TestUint64SliceSort(t *testing.T) {
	s := uint64Slice{3, 1, 2}
	sort.Sort(s)
	if s[0] != 1 || s[1] != 2 || s[2] != 3 {
		t.Errorf("sorted slice = %v, want [1 2 3]", []uint64(s))
	}
}

func TestMaxMin(t *testing.T) {
	if max(2, 1) != 2 || max(1, 2) != 2 {
		t.Error("max returned the wrong value")
	}
	if min(2, 1) != 1 || min(1, 2) != 1 {
		t.Error("min returned the wrong value")
	}
}

func TestProgressMaybeUpdate(t *testing.T) {
	pr := &Progress{Match: 1, Next: 2}

	if pr.maybeUpdate(3) != true {
		t.Error("maybeUpdate(3) should report an update")
	}
	if pr.Match != 3 || pr.Next != 4 {
		t.Errorf("progress = (Match: %d, Next: %d), want (3, 4)", pr.Match, pr.Next)
	}

	if pr.maybeUpdate(2) != false {
		t.Error("maybeUpdate(2) should not report an update")
	}
}

func TestProgressMayDecrTo(t *testing.T) {
	pr := &Progress{Match: 0, Next: 5}

	if pr.mayDecrTo(3, 1) != false {
		t.Error("mayDecrTo(3, 1) should not apply when Next-1 does not match")
	}
	if pr.mayDecrTo(4, 1) != true {
		t.Error("mayDecrTo(4, 1) should apply")
	}
	if pr.Next != 2 {
		t.Errorf("Next = %d, want 2", pr.Next)
	}
}

func TestRaftLogTermFromUnstable(t *testing.T) {
	l := newRaftLog(NewMemoryStorage())
	l.append(Entry{Index: 1, Term: 1}, Entry{Index: 2, Term: 2})

	// Terms of entries that have not been persisted yet must be readable.
	got, err := l.term(2)
	if err != nil || got != 2 {
		t.Fatalf("term(2) = (%d, %v), want (2, nil)", got, err)
	}
}

func TestRaftLogIsUpToDate(t *testing.T) {
	l := newRaftLog(NewMemoryStorage())
	l.append(Entry{Index: 1, Term: 1}, Entry{Index: 2, Term: 2})

	// Same term, shorter log is not up to date.
	if l.isUpToDate(1, 2) {
		t.Error("isUpToDate(1, 2) = true, want false")
	}
	// Same term, same length is up to date.
	if !l.isUpToDate(2, 2) {
		t.Error("isUpToDate(2, 2) = false, want true")
	}
	// A higher term is always up to date.
	if !l.isUpToDate(0, 3) {
		t.Error("isUpToDate(0, 3) = false, want true")
	}
}

func TestRaftLogMaybeAppend(t *testing.T) {
	l := newRaftLog(NewMemoryStorage())
	l.append(Entry{Index: 1, Term: 1}, Entry{Index: 2, Term: 1})

	// Appending entries that fit onto the local log succeeds.
	last, ok := l.maybeAppend(2, 1, 2, Entry{Index: 3, Term: 2})
	if !ok || last != 3 {
		t.Fatalf("maybeAppend = (%d, %v), want (3, true)", last, ok)
	}
	if l.commitIndex != 2 {
		t.Errorf("commitIndex = %d, want 2", l.commitIndex)
	}

	// Appending with a mismatched preceding entry fails.
	if _, ok := l.maybeAppend(2, 2, 0, Entry{Index: 3, Term: 3}); ok {
		t.Fatal("maybeAppend with a wrong preceding term should fail")
	}
}

func TestRaftLogMaybeAppendNoEntries(t *testing.T) {
	l := newRaftLog(NewMemoryStorage())
	l.append(Entry{Index: 1, Term: 1})

	// Appending zero entries must not panic.
	last, ok := l.maybeAppend(1, 1, 0)
	if !ok || last != 1 {
		t.Fatalf("maybeAppend = (%d, %v), want (1, true)", last, ok)
	}
}

func TestRaftLogNextEnts(t *testing.T) {
	l := newRaftLog(NewMemoryStorage())
	l.append(Entry{Index: 1, Term: 1}, Entry{Index: 2, Term: 1})

	l.commitTo(2)
	ents := l.nextEnts()
	if len(ents) != 2 || ents[0].Index != 1 || ents[1].Index != 2 {
		t.Fatalf("nextEnts = %v, want entries [1 2]", ents)
	}

	l.appliedTo(2)
	if ents := l.nextEnts(); ents != nil {
		t.Fatalf("nextEnts after applying = %v, want nil", ents)
	}
}
