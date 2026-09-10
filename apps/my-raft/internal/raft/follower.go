package raft

func (r *raft) becomeFollower(term, lead uint64) {
	r.reset(term)
	r.step = stepFollower
	r.tick = r.tickElection
	r.lead = lead
	r.state = StateFollower
}

// stepFollower is the state-machine function of the follower role.
func stepFollower(r *raft, m Message) {
	switch m.Type {
	case MsgProp:
		// Without a leader there is nobody to forward to.
		if r.lead == None {
			return
		}

		// Forward the proposal to the leader.
		m.To = r.lead
		r.send(m)

	case MsgApp:
		// Handle the replication request.
		// Message from the leader: reset the election timer.
		r.electionElapsed = 0
		r.lead = m.From
		r.handleAppendEntries(m)
	case MsgHeartbeat:
		// Handle the heartbeat request.
		// Message from the leader: reset the election timer.
		r.electionElapsed = 0
		r.lead = m.From
		r.handleHeartbeat(m)
	case MsgReadIndex:
		// Handle the read request.
		// Ignore it if there is no leader.
		if r.lead == None {
			return
		}

		// Otherwise forward it to the leader.
		m.To = r.lead
		r.send(m)

	case MsgReadIndexResp:
		// Handle the read request response.
		r.readStates = append(r.readStates, ReadState{Index: m.LogIndex, RequestCtx: m.Entries[0].Data})
	}
}

func (r *raft) handleAppendEntries(m Message) {
	// If these entries were already committed locally, reply with the commit
	// index directly.
	if m.LogIndex < r.raftLog.commitIndex {
		r.send(Message{To: m.From, Type: MsgAppResp, LogIndex: r.raftLog.commitIndex})
		return
	}

	// Try to append; reject on failure.
	if mLastIndex, ok := r.raftLog.maybeAppend(m.LogIndex, m.LogTerm, m.CommitIndex, m.Entries...); ok {
		r.send(Message{To: m.From, Type: MsgAppResp, LogIndex: mLastIndex})
		return
	}

	// Append failed.
	r.send(Message{To: m.From, Type: MsgAppResp, LogIndex: m.LogIndex, Reject: true, RejectHint: r.raftLog.lastIndex()})
}

func (r *raft) handleHeartbeat(m Message) {
	r.raftLog.commitTo(m.CommitIndex)
	r.send(Message{To: m.From, Type: MsgHeartbeatResp, Context: m.Context})
}
