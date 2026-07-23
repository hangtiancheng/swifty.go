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

	// Newly elected leader appends a no-op entry for the current term
	r.appendEntry([]Entry{{Data: nil}}...)
}

func stepLeader(r *raft, m Message) {
	switch m.Type {
	case MsgBeat:
		// Broadcast heartbeats to all followers
		r.broadcastHeartbeat()
		return
		// Handle write proposal
	case MsgProp:
		if len(m.Entries) == 0 {
			panic("propose entries can not be empty")
		}
		// Verify this node is still in the cluster
		if _, ok := r.prs[r.id]; !ok {
			return
		}

		// Check for pending configuration changes

		// Append entries to the local log first
		r.appendEntry(m.Entries...)

		// Broadcast append to all followers
		r.broadcastAppend()
		return
	case MsgReadIndex:
		// Handle linearizable read request
		return
	}

	// Handle response messages
	// Verify the sender is a known cluster member
	pr, ok := r.prs[m.From]
	if !ok {
		return
	}

	switch m.Type {
	case MsgAppResp:
		// If the append request was rejected
		if m.Reject {
			// Retry with a lower index based on the reject hint
			if pr.mayDecreaseTo(m.LogIndex, m.RejectHint) {
				r.sendAppend(m.From)
			}
		}

	case MsgHeartbeatResp:

	}

}

func (r *raft) broadcastHeartbeat() {
	r.broadcastHeartbeatWithCtx(nil)
}

func (r *raft) broadcastHeartbeatWithCtx(ctx []byte) {
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

func (r *raft) broadcastAppend() {
	for id := range r.prs {
		if id == r.id {
			continue
		}
		r.sendAppend(id)
	}
}

func (r *raft) sendAppend(to uint64) {
	// Retrieve the term and index of the preceding entry
	pr := r.prs[to]
	term, _ := r.raftLog.term(pr.Next - 1)
	entries, _ := r.raftLog.entries(pr.Next)
	m := Message{
		To:          to,
		Type:        MsgApp,
		LogTerm:     term,
		LogIndex:    pr.Next - 1,
		Entries:     entries,
		CommitIndex: r.raftLog.commitIndex,
	}
	r.send(m)
}
