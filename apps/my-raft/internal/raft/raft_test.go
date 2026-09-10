package raft

import (
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

// testCluster is a deterministic, tick-driven Raft cluster with no goroutines:
// messages stay in the sender's outbox until deliver routes them.
type testCluster struct {
	t     *testing.T
	nodes map[uint64]*raft
}

func newTestCluster(t *testing.T, ids ...uint64) *testCluster {
	t.Helper()
	c := &testCluster{t: t, nodes: make(map[uint64]*raft, len(ids))}
	for _, id := range ids {
		conf := &Config{
			ID:            id,
			ElectionTick:  10,
			HeartbeatTick: 1,
			Storage:       NewMemoryStorage(),
		}
		conf.peers = append([]uint64(nil), ids...)
		c.nodes[id] = newRaft(conf)
	}
	return c
}

func (c *testCluster) get(id uint64) *raft {
	return c.nodes[id]
}

// deliver routes every pending message to its recipient, repeatedly, until
// no node has anything left in its outbox.
func (c *testCluster) deliver() {
	for {
		moved := false
		for _, r := range c.nodes {
			for len(r.msgs) > 0 {
				m := r.msgs[0]
				r.msgs = r.msgs[1:]
				moved = true
				if to, ok := c.nodes[m.To]; ok {
					if err := to.Step(m); err != nil {
						c.t.Fatalf("Step(%v): %v", m, err)
					}
				}
			}
		}
		if !moved {
			return
		}
	}
}

// elect ticks the given node until it wins the election, then delivers the
// resulting replication traffic. Messages are routed after every tick, which
// mirrors a network faster than the election timeout.
func (c *testCluster) elect(id uint64) {
	c.t.Helper()
	r := c.get(id)
	for range 50 {
		r.tick()
		c.deliver()
		if r.state == StateLeader {
			break
		}
	}
	if r.state != StateLeader {
		c.t.Fatalf("node %d did not become leader, state: %v", id, r.state)
	}
}

func (c *testCluster) propose(id uint64, data string) {
	c.t.Helper()
	if err := c.get(id).Step(Message{
		From:    id,
		Type:    MsgProp,
		Entries: []Entry{{Data: []byte(data)}},
	}); err != nil {
		c.t.Fatalf("propose: %v", err)
	}
}

func (c *testCluster) commitIndex(id uint64) uint64 {
	return c.get(id).raftLog.commitIndex
}

func TestResetClearsVotesBetweenCampaigns(t *testing.T) {
	c := newTestCluster(t, 1, 2, 3)
	r := c.get(1)

	r.campaign(campaignElection)
	r.poll(2, true)

	// A new campaign must start from a clean slate: leaked votes from the
	// previous campaign would let a single response reach the quorum.
	r.campaign(campaignElection)
	if r.state != StateCandidate {
		t.Fatalf("state = %v, want candidate (stale votes reached the quorum)", r.state)
	}
	if len(r.votes) != 1 || !r.votes[1] {
		t.Fatalf("votes = %v, want only the self vote", r.votes)
	}
}

func TestSendStampsFrom(t *testing.T) {
	c := newTestCluster(t, 1, 2)
	r := c.get(1)
	r.campaign(campaignElection)

	for _, m := range r.msgs {
		if m.From != 1 {
			t.Fatalf("message %+v has From = %d, want the sender ID 1", m, m.From)
		}
	}
}

func TestSingleNodeElectionAndCommit(t *testing.T) {
	c := newTestCluster(t, 1)
	c.elect(1)

	// The no-op entry the leader appends on election is committed at once.
	if got := c.commitIndex(1); got != 1 {
		t.Fatalf("commitIndex = %d, want 1", got)
	}

	c.propose(1, "hello")
	c.deliver()

	if got := c.commitIndex(1); got != 2 {
		t.Fatalf("commitIndex = %d, want 2", got)
	}
	ents := c.get(1).raftLog.nextEnts()
	if len(ents) != 2 || string(ents[1].Data) != "hello" {
		t.Fatalf("nextEnts = %v, want the no-op and the proposal", ents)
	}
}

func TestClusterElectsLeaderAndReplicates(t *testing.T) {
	c := newTestCluster(t, 1, 2, 3)
	c.elect(1)

	if lead := c.get(2).lead; lead != 1 {
		t.Fatalf("node 2 lead = %d, want 1", lead)
	}

	c.propose(1, "x")
	c.deliver()

	// The quorum acknowledges the entry, so it is committed and replicated
	// with the commit index everywhere.
	for _, id := range []uint64{1, 2, 3} {
		if got := c.commitIndex(id); got != 2 {
			t.Fatalf("node %d commitIndex = %d, want 2", id, got)
		}
		if c.get(id).raftLog.lastIndex() != 2 {
			t.Fatalf("node %d lastIndex = %d, want 2", id, c.get(id).raftLog.lastIndex())
		}
	}
	if ents := c.get(3).raftLog.nextEnts(); len(ents) != 2 || string(ents[1].Data) != "x" {
		t.Fatalf("node 3 nextEnts = %v, want the no-op and the proposal", ents)
	}
}

func TestReadIndexQuorumAck(t *testing.T) {
	c := newTestCluster(t, 1, 2, 3)
	c.elect(1)
	c.propose(1, "x")
	c.deliver()

	ctx := []byte("read-1")
	if err := c.get(1).Step(Message{Type: MsgReadIndex, Context: ctx}); err != nil {
		t.Fatalf("Step read index: %v", err)
	}
	c.deliver()

	states := c.get(1).readStates
	if len(states) != 1 {
		t.Fatalf("got %d read states, want 1", len(states))
	}
	if states[0].Index != 2 || string(states[0].RequestCtx) != "read-1" {
		t.Fatalf("read state = %+v, want {Index: 2, RequestCtx: read-1}", states[0])
	}
}

func TestMemoryStorageAppendTruncatesConflict(t *testing.T) {
	ms := NewMemoryStorage()
	if err := ms.Append([]Entry{
		{Index: 1, Term: 1},
		{Index: 2, Term: 1},
		{Index: 3, Term: 1},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Entries that overlap the persisted tail replace it.
	if err := ms.Append([]Entry{
		{Index: 2, Term: 2},
		{Index: 3, Term: 2},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got, _ := ms.Term(2); got != 2 {
		t.Fatalf("Term(2) = %d, want 2", got)
	}
	if got, _ := ms.LastIndex(); got != 3 {
		t.Fatalf("LastIndex = %d, want 3", got)
	}

	// Appending beyond a gap is rejected instead of creating a hole.
	if err := ms.Append([]Entry{{Index: 5, Term: 2}}); err == nil {
		t.Fatal("expected an error when appending across a gap")
	}
}
