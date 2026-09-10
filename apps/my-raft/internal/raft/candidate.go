package raft

func (r *raft) becomePreCandidate() {
	if r.state == StateLeader {
		panic("invalid transition leader -> pre-candidate")
	}
	// The term and the vote are intentionally left untouched, but stale votes
	// from previous campaigns must not leak into this one.
	r.reset(r.Term)
	r.step = stepCandidate
	r.tick = r.tickElection
	r.state = StatePreCandidate
}

func (r *raft) becomeCandidate() {
	if r.state == StateLeader {
		panic("invalid transition leader -> candidate")
	}
	r.step = stepCandidate
	r.reset(r.Term + 1)
	r.tick = r.tickElection
	// A candidate votes for itself first.
	r.Vote = r.id
	r.state = StateCandidate
}

// stepCandidate is the state-machine function of the candidate role.
func stepCandidate(r *raft, m Message) {
	var voteRespType MessageType
	if r.state == StatePreCandidate {
		voteRespType = MsgPreVoteResp
	} else {
		voteRespType = MsgVoteResp
	}

	switch m.Type {
	case MsgProp:
		// A candidate does not accept write proposals.
		return
	case MsgApp:
		// A replication request from a leader with a higher term makes us
		// step back down to follower.
		r.becomeFollower(r.Term, m.From)
		// Handle the replication request.
		r.handleAppendEntries(m)
	case MsgHeartbeat:
		r.becomeFollower(m.Term, m.From)
		// Update the commit index from the heartbeat.
		r.handleHeartbeat(m)
	case voteRespType:
		// Tally the votes received so far.
		granted := r.poll(m.From, !m.Reject)
		switch r.quorum() {
		// Won a majority of the votes.
		case granted:
			if r.state == StatePreCandidate {
				r.campaign(campaignElection)
			} else {
				r.becomeLeader()
				// After becoming leader, broadcast to replicate the existing
				// log entries; becomeLeader has already appended an empty
				// entry for the current term.
				r.bcastAppend()
			}
		// Rejected by a majority.
		case len(r.votes) - granted:
			// Step back down to follower.
			r.becomeFollower(r.Term, None)
		}
	}
}
