package raft

func (r *raft) becomeLeader() {
	if r.state == StateFollower {
		panic("invalid transition [follower -> leader]")
	}
	r.step = stepLeader
	r.reset(r.Term)
	r.tick = r.tickHeartbeat
	r.lead = r.id
	r.state = StateLeader

	// A newly elected leader appends an empty entry for the current term.
	r.appendEntry([]Entry{{Data: nil}}...)
}

func stepLeader(r *raft, m Message) {
	switch m.Type {
	case MsgBeat:
		// Broadcast heartbeats.
		r.bcastHeartbeat()
		return
	case MsgProp:
		// Write proposal.
		if len(m.Entries) == 0 {
			panic("propose entries can not be empty")
		}
		// Check that this node is still in the cluster.
		if _, ok := r.prs[r.id]; !ok {
			return
		}

		// Check for configuration change entries.

		// First append the entries to the local log.
		r.appendEntry(m.Entries...)

		// Broadcast to replicate the new entries.
		r.bcastAppend()
		return
	case MsgReadIndex:
		// Handle the read request.
		return
	}

	// Handle response-type messages.
	// First check that the sender is still in the cluster.
	pr, ok := r.prs[m.From]
	if !ok {
		return
	}

	switch m.Type {
	case MsgAppResp:
		// The replication request was rejected.
		if m.Reject {
			// Send new entries based on the reject hint.
			if pr.mayDecrTo(m.LogIndex, m.RejectHint) {
				r.sendAppend(m.From)
			}
		}

	case MsgHeartbeatResp:
	}

}

func (r *raft) bcastHeartbeat() {
	r.bcastHeartbeatWithCtx(nil)
}

func (r *raft) bcastHeartbeatWithCtx(ctx []byte) {
	for id := range r.prs {
		if id == r.id {
			continue
		}
		r.sendHeartbeat(id, ctx)
	}
}

func (r *raft) sendHeartbeat(id uint64, ctx []byte) {
	commit := min(r.raftLog.commitIndex, r.prs[id].Match)

	m := Message{
		To:          id,
		Type:        MsgHeartbeat,
		CommitIndex: commit,
		Context:     ctx,
	}

	r.send(m)
}

func (r *raft) bcastAppend() {
	for id := range r.prs {
		if id == r.id {
			continue
		}
		r.sendAppend(id)
	}
}

func (r *raft) sendAppend(to uint64) {
	// Get the term and index of the entry preceding the ones to send.
	pr := r.prs[to]
	term, _ := r.raftLog.term(pr.Next - 1)
	ents, _ := r.raftLog.entries(pr.Next)
	m := Message{
		To:          to,
		Type:        MsgApp,
		LogTerm:     term,
		LogIndex:    pr.Next - 1,
		Entries:     ents,
		CommitIndex: r.raftLog.commitIndex,
	}
	r.send(m)
}
