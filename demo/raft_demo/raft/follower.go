package raft

func (r *raft) becomeFollower(term, lead uint64) {
	r.reset(term)
	r.step = stepFollower
	r.tick = r.tickElection
	r.lead = lead
	r.state = StateFollower
}

// State machine handler for the follower role
func stepFollower(r *raft, m Message) {
	switch m.Type {
	case MsgProp:
		// Ignore if no leader is known
		if r.lead == None {
			return
		}

		// Forward to the known leader
		m.To = r.lead
		r.send(m)

	case MsgApp:
		// Handle log replication request
		// Reset election timer upon receiving leader's append request
		r.electionElapsed = 0
		r.lead = m.From
		r.handleAppendEntries(m)
	case MsgHeartbeat:
		// Handle heartbeat request
		// Reset election timer upon receiving leader's heartbeat
		r.electionElapsed = 0
		r.lead = m.From
		r.handleHeartbeat(m)
	case MsgReadIndex:
		// Handle linearizable read request
		// Ignore if no leader is known
		if r.lead == None {
			return
		}

		// Forward to the leader
		m.To = r.lead
		r.send(m)

	case MsgReadIndexResp:
		// Handle read index response
		r.readStates = append(r.readStates, ReadState{Index: m.LogIndex, RequestCtx: m.Entries[0].Data})
	}
}

func (r *raft) handleAppendEntries(m Message) {
	// Ignore entries already committed
	if m.LogIndex < r.raftLog.commitIndex {
		r.send(Message{To: m.From, Type: MsgAppResp, LogIndex: r.raftLog.commitIndex})
		return
	}

	// Attempt to append; reject on failure
	if mLastIndex, ok := r.raftLog.maybeAppend(m.LogIndex, m.LogIndex, m.CommitIndex, m.Entries...); ok {
		r.send(Message{To: m.From, Type: MsgAppResp, LogIndex: mLastIndex})
		return
	}

	// Append failed, send rejection with hint
	r.send(Message{To: m.From, Type: MsgAppResp, LogIndex: m.LogIndex, Reject: true, RejectHint: r.raftLog.lastIndex()})
}

func (r *raft) handleHeartbeat(m Message) {
	r.raftLog.commitTo(m.CommitIndex)
	r.send(Message{To: m.From, Type: MsgHeartbeatResp, Context: m.Context})
}
