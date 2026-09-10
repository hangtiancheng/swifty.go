package raft

import (
	"math/rand"
	"slices"
)

type stepFunc func(*raft, Message)

type raft struct {
	// ID of this node.
	id uint64
	// Current term.
	Term uint64
	// Results of linearizable read requests.
	readStates []ReadState
	// The log module.
	raftLog *raftLog
	// Replication progress of each node.
	prs map[uint64]*Progress
	// Current role of this node.
	state StateType
	// Records which nodes voted for this node.
	votes map[uint64]bool
	// Reads tracks the in-flight ReadIndex requests by their context,
	// counting the quorum confirmations received so far.
	reads map[string]int
	// Messages waiting to be delivered to the application.
	msgs []Message
	// ID of the current leader.
	lead uint64
	// Whether the node runs the pre-vote phase before elections.
	preVote bool
	// tick is executed whenever the timer fires; each role has its own logic.
	tick func()
	// step is the state-machine function of the current role.
	step stepFunc
	// ID of the node this node voted for.
	Vote uint64
	// Whether to check quorum to guard against a partitioned minority.
	checkQuorum bool
	// Election timeout, in ticks.
	electionTimeout int32
	// Randomized election timeout, in ticks.
	randomizedElectionTimeout int32
	// Election tick counter.
	electionElapsed int32
	// Heartbeat timeout, in ticks.
	heartbeatTimeout int32
	// Heartbeat tick counter.
	heartbeatElapsed int32
}

func newRaft(conf *Config) *raft {
	// Fetch the configuration and hard state from the storage module.
	// hs, cs, err := conf.Storage.InitialState()
	// if err != nil {
	// 	panic(err)
	// }

	// Build the raft object from the configuration.
	r := raft{
		id:               conf.ID,
		lead:             None,
		prs:              make(map[uint64]*Progress),
		votes:            make(map[uint64]bool),
		raftLog:          newRaftLog(conf.Storage),
		electionTimeout:  conf.ElectionTick,
		heartbeatTimeout: conf.HeartbeatTick,
		preVote:          conf.Prevote,
	}

	// Add the peers to the progress map.
	for _, peer := range conf.peers {
		r.prs[peer] = &Progress{Next: 1}
	}

	// Update the applied index.
	r.raftLog.appliedTo(conf.Applied)

	// Start in the follower state.
	r.becomeFollower(1, None)

	return &r
}

func (r *raft) Step(m Message) error {
	// The first switch dispatches on the term of the message.
	switch {
	// Local messages pass through directly.
	case m.Term == 0:
	case m.Term > r.Term:
		lead := m.From
		// The message carries a newer term.
		if m.Type == MsgVote || m.Type == MsgPreVote {
			lead = None
		}

		if m.Type != MsgPreVote && (m.Type != MsgPreVoteResp || m.Reject) {
			r.becomeFollower(m.Term, lead)
		}

	case m.Term < r.Term:
		// The message carries an older term.
		// Heartbeats and replication requests must inform the sender of the newer term.
		if r.checkQuorum && (m.Type == MsgHeartbeat || m.Type == MsgApp) {
			r.send(Message{To: m.From, Type: MsgAppResp})
		}
		// Messages with an older term can always be ignored.
		return nil
	}

	// The second switch dispatches on the message type.
	switch m.Type {
	// Drive this node to take part in an election.
	case MsgHup:
		// Already the leader, nothing to do.
		if r.state == StateLeader {
			break
		}
		// Do not start an election while there are committed but unapplied
		// configuration change entries.
		ents, err := r.raftLog.slice(r.raftLog.applyIndex+1, r.raftLog.commitIndex+1)
		if err != nil {
			panic(err)
		}

		if n := numOfPendingConf(ents); n > 0 {
			break
		}

		// Run the election.
		if r.preVote {
			// Run the pre-vote phase first.
			r.campaign(campaignPreElection)
			break
		}
		r.campaign(campaignElection)

	// Received a vote request.
	case MsgVote, MsgPreVote:
		// A vote request whose term is greater than or equal to ours: grant
		// it if the candidate's log is at least as up to date as ours and one
		// of the following holds: (1) we have not voted yet, (2) the campaign
		// term is greater than ours, or (3) the candidate is the node we
		// already voted for.
		if r.raftLog.isUpToDate(m.LogIndex, m.LogTerm) && (r.Vote == None || m.Term > r.Term || m.From == r.Vote) {
			if m.Type == MsgVote {
				// Granting a vote resets the election timer.
				r.electionElapsed = 0
				r.Vote = m.From
				r.send(Message{Type: MsgVoteResp, To: m.From})
				break
			}
			r.send(Message{Type: MsgPreVoteResp, To: m.From})
			break
		}
		if m.Type == MsgVote {
			r.send(Message{Type: MsgVoteResp, To: m.From, Reject: true})
			break
		}
		r.send(Message{Type: MsgPreVoteResp, To: m.From, Reject: true})

	// Everything else goes to the role-specific state-machine function.
	default:
		r.step(r, m)
	}

	return nil
}

func (r *raft) reset(term uint64) {
	if r.Term != term {
		r.Term = term
		r.Vote = None
	}
	r.lead = None
	r.electionElapsed = 0
	r.heartbeatElapsed = 0
	// Reset the randomized election timeout.
	r.resetRandomizedElectionTimeout()
	// Votes from previous campaigns must never leak into a new one.
	r.votes = make(map[uint64]bool)
	// Read requests confirmed by a previous leadership are stale.
	r.reads = make(map[string]int)
	// Rebuild the replication progress: the new leadership restarts from the
	// local log end, and stale Match values must not be counted as quorum.
	for id := range r.prs {
		r.prs[id] = &Progress{Next: r.raftLog.lastIndex() + 1}
	}
}

func (r *raft) resetRandomizedElectionTimeout() {
	r.randomizedElectionTimeout = r.electionTimeout + int32(rand.Intn(int(r.electionTimeout)))
}

func (r *raft) softState() *SoftState {
	return &SoftState{Lead: r.lead, RaftState: r.state}
}

func (r *raft) hardState() HardState {
	return HardState{
		Term:        r.Term,
		CommitIndex: r.raftLog.commitIndex,
		Vote:        r.Vote,
	}
}

func (r *raft) send(m Message) {
	if m.Type != MsgProp && m.Type != MsgReadIndex {
		m.Term = r.Term
	}
	// Outgoing messages must carry the sender ID: receivers use it to look up
	// the sender's progress and to learn the leader.
	if m.From == None {
		m.From = r.id
	}

	r.msgs = append(r.msgs, m)
}

func (r *raft) campaign(typ CampaignType) {
	// A campaign actually consists of two phases: pre-vote and vote.
	var (
		term    uint64
		msgType MessageType
	)

	if typ == campaignPreElection {
		// During the pre-vote phase the raft term is not incremented.
		r.becomePreCandidate()
		term = r.Term + 1
		msgType = MsgPreVote
	} else {
		// During the vote phase the raft term is incremented.
		r.becomeCandidate()
		term = r.Term
		msgType = MsgVote
	}

	// As a candidate, vote for ourselves first and check whether that
	// already forms a quorum.
	if r.quorum() == r.poll(r.id, true) {
		if typ == campaignPreElection {
			r.campaign(campaignElection)
		} else {
			r.becomeLeader()
		}
		return
	}

	// Solicit votes from the rest of the cluster.
	for id := range r.prs {
		// No need to send to ourselves.
		if id == r.id {
			continue
		}

		r.send(Message{Term: term, To: id, Type: msgType, LogTerm: r.raftLog.lastTerm(), LogIndex: r.raftLog.lastIndex()})
	}
}

func (r *raft) poll(id uint64, v bool) int {
	// If id has not voted yet, record its vote.
	if _, ok := r.votes[id]; !ok {
		r.votes[id] = v
	}
	var granted int
	for _, vv := range r.votes {
		if vv {
			granted++
		}
	}
	return granted
}

func (r *raft) quorum() int {
	return len(r.prs)>>1 + 1
}

func (r *raft) tickElection() {
	r.electionElapsed++

	if r.promotable(r.id) && r.pastElectionTimeout() {
		r.electionElapsed = 0
		r.Step(Message{From: r.id, Type: MsgHup})
	}
}

func (r *raft) tickHeartbeat() {
	if r.state != StateLeader {
		return
	}

	r.heartbeatElapsed++
	if r.heartbeatElapsed >= r.heartbeatTimeout {
		r.heartbeatElapsed = 0
		r.Step(Message{From: r.id, Type: MsgBeat})
	}
}

// promotable reports whether the node is still a member of the cluster.
func (r *raft) promotable(id uint64) bool {
	_, ok := r.prs[id]
	return ok
}

func (r *raft) pastElectionTimeout() bool {
	return r.electionElapsed >= r.randomizedElectionTimeout
}

func (r *raft) appendEntry(es ...Entry) {
	lastIndex := r.raftLog.lastIndex()
	for i := range es {
		es[i].Term = r.Term
		es[i].Index = lastIndex + uint64(i) + 1
	}
	// Assign the term and index of the new entries.
	r.raftLog.append(es...)
	r.prs[r.id].maybeUpdate(r.raftLog.lastIndex())
}

// maybeCommit advances the commit index to the highest index replicated on a
// quorum of nodes, if it is higher than the current commit index.
func (r *raft) maybeCommit() bool {
	matches := make([]uint64, 0, len(r.prs))
	for _, pr := range r.prs {
		matches = append(matches, pr.Match)
	}
	slices.Sort(matches)
	// The majority-matched index is the middle of the sorted matches.
	return r.raftLog.maybeCommit(matches[len(matches)-1-(len(matches)-1)/2])
}
