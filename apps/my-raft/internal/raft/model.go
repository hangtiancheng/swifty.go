package raft

import "math"

const (
	None    uint64 = 0
	noLimit uint64 = math.MaxUint64
)

type EntryType int32

const (
	// EntryNormal is a regular log entry.
	EntryNormal EntryType = 0
	// EntryConfChange is a configuration change log entry.
	EntryConfChange EntryType = 1
)

type Entry struct {
	Term  uint64    `json:"term"`
	Index uint64    `json:"index"`
	Type  EntryType `json:"type"`
	Data  []byte    `json:"data"`
}

type MessageType int32

const (
	// MsgHup drives the local node into an election.
	MsgHup MessageType = 0
	// MsgBeat drives the leader to broadcast heartbeats.
	MsgBeat MessageType = 1
	// MsgProp is a proposal submitted by the local user.
	MsgProp MessageType = 2
	// MsgApp is sent by the leader to replicate log entries to other nodes.
	MsgApp MessageType = 3
	// MsgAppResp is the follower's response to MsgApp.
	MsgAppResp MessageType = 4
	// MsgVote / MsgVoteResp are the vote request and response.
	MsgVote     MessageType = 5
	MsgVoteResp MessageType = 6
	// MsgHeartbeat / MsgHeartbeatResp are the heartbeat request and response.
	MsgHeartbeat     MessageType = 7
	MsgHeartbeatResp MessageType = 8
	// MsgReadIndex / MsgReadIndexResp carry linearizable read requests.
	MsgReadIndex     MessageType = 9
	MsgReadIndexResp MessageType = 10
	// MsgPreVote / MsgPreVoteResp are the pre-vote request and response.
	MsgPreVote     MessageType = 11
	MsgPreVoteResp MessageType = 12
)

type Message struct {
	Type MessageType `json:"type"`
	To   uint64      `json:"to"`
	From uint64      `json:"from"`
	// Term of the message.
	Term uint64 `json:"term"`
	// Term of the entry preceding the ones being sent.
	LogTerm  uint64 `json:"logTerm"`
	LogIndex uint64 `json:"logIndex"`
	// Entries being replicated.
	Entries []Entry `json:"entries"`
	// Highest committed index known to the sender.
	CommitIndex uint64 `json:"commitIndex"`
	// Reject indicates that the request is refused.
	Reject bool `json:"reject"`
	// Best hint of the sender's log when a replication request is rejected.
	RejectHint uint64 `json:"rejectHint"`
	// Opaque context attached to the message.
	Context []byte `json:"context"`
}

type StateType int32

const (
	// StateFollower is the follower role.
	StateFollower StateType = 0
	// StateCandidate is the candidate role.
	StateCandidate StateType = 1
	// StateLeader is the leader role.
	StateLeader StateType = 2
	// StatePreCandidate is the pre-candidate role.
	StatePreCandidate StateType = 3
)

type SoftState struct {
	// Lead is the current leader of the cluster.
	Lead uint64
	// RaftState is the current role of this node.
	RaftState StateType
}

func (s *SoftState) equal(pre *SoftState) bool {
	return s.Lead == pre.Lead && s.RaftState == pre.RaftState
}

var emptyHardState HardState

type HardState struct {
	// Term is the current term.
	Term uint64 `json:"term"`
	// Vote is the node voted for in the current term.
	Vote uint64 `json:"vote"`
	// CommitIndex is the index of the highest committed entry.
	CommitIndex uint64 `json:"commitIndex"`
}

func isHardStateEqual(a, b HardState) bool {
	return a.Term == b.Term && a.Vote == b.Vote && a.CommitIndex == b.CommitIndex
}

type ConfState struct {
	// IDs of the nodes in the cluster.
	Nodes []uint64
}

type Config struct {
	// ID of this node.
	ID uint64
	// IDs of the other nodes.
	peers []uint64
	// Persistent storage interface.
	Storage Storage
	// Applied is the index of the highest applied entry.
	Applied uint64
	// Prevote is whether to run the pre-vote phase before elections.
	Prevote bool
	// ElectionTick is the election timeout, in ticks, for a follower.
	ElectionTick int32
	// HeartbeatTick is the heartbeat interval, in ticks, for the leader.
	HeartbeatTick int32
}

type Peer struct {
	// ID of the peer.
	ID uint64
	// Opaque context for the peer.
	Context []byte
}

func max(l, r uint64) uint64 {
	if l > r {
		return l
	}
	return r
}

func min(l, r uint64) uint64 {
	if l < r {
		return l
	}
	return r
}

type CampaignType string

const (
	campaignPreElection CampaignType = "prev"
	campaignElection    CampaignType = "nor"
)

type ConfChangeType int32

const (
	ConfChangeAddNode    ConfChangeType = 0
	ConfChangeRemoveNode ConfChangeType = 1
	ConfChangeUpdateNode ConfChangeType = 2
)

type ConfChange struct {
	ID      uint64         `json:"id"`
	Type    ConfChangeType `json:"type"`
	NodeID  uint64         `json:"nodeID"`
	Context []byte         `json:"context"`
}
