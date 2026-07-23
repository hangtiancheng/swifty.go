package raft

import (
	"context"
	"encoding/json"
)

type Node struct {
	// Channel for local proposals
	proc chan Message
	// Channel for messages from peers
	recvChan chan Message
	// Channel for configuration change proposals
	confChan chan Message
	// Channel delivering ready state to the application
	readyChan chan Ready
	// Channel signaling the application has applied ready state
	advanceChan chan struct{}
	// Tick channel
	tickChan chan struct{}
}

func StartNode(conf *Config, peers []Peer) Node {
	r := newRaft(conf)
	// Register all peers in the configuration
	// for _, peer := range peers {

	// }
	r.raftLog.commitIndex = r.raftLog.lastIndex()

	// Initialize progress for each peer
	// for _, peer := range peers {
	// 	r.addNode(peer.ID)
	// }

	n := newNode()
	go n.run(r)
	return n
}

func newNode() Node {
	return Node{
		proc:   make(chan Message),
		recvChan:  make(chan Message),
		confChan:  make(chan Message),
		readyChan: make(chan Ready),
		tickChan:  make(chan struct{}),
	}
}

func (n *Node) run(r *raft) {
	var (
		propChan    chan Message
		readyChan   chan Ready
		advanceChan chan struct{}
		// Ready state
		rd       Ready
		prevSoft = r.softState()
		prevHard = emptyHardState

		prevLastUnstableI, prevLastUnstableJ uint64
		hasPrevLastUnstableI                  bool
	)

	for {
		// Check for state updates and deliver via readyChan if present
		if advanceChan != nil {
			readyChan = nil
			// Build new ready state and deliver if there are updates
		} else if rd = newReady(r, prevSoft, prevHard); rd.containsUpdates() {
			readyChan = n.readyChan
		} else {
			readyChan = nil
		}

		select {
		// Received a local proposal
		case m := <-propChan:
			// Process local proposal message
			m.From = r.id
			r.Step(m)
		case m := <-n.recvChan:
			// Ignore messages from unknown nodes
			if _, ok := r.prs[m.From]; !ok || !IsResponseMsg(m.Type) {
				break
			}
			r.Step(m)
		case <-n.confChan:
		// Timer tick
		case <-n.tickChan:
			r.tick()
			// Ready state delivered
		case readyChan <- rd:
			// Sync updated state for next iteration's change detection
			if rd.SoftState != nil {
				prevSoft = rd.SoftState
			}

			if !IsEmptyHardState(rd.HardState) {
				prevHard = rd.HardState
			}

			if len(rd.Entries) > 0 {
				prevLastUnstableI, prevLastUnstableJ = rd.Entries[len(rd.Entries)-1].Index, rd.Entries[len(rd.Entries)-1].Term
				hasPrevLastUnstableI = true
			}

			r.msgs = nil
			r.readStates = nil
			advanceChan = n.advanceChan
		case <-advanceChan:
			// Previous commit index is now applied, as the application signaled advance
			if prevHard.CommitIndex != 0 {
				r.raftLog.appliedTo(prevHard.CommitIndex)
			}

			if hasPrevLastUnstableI {
				r.raftLog.stableTo(prevLastUnstableI, prevLastUnstableJ)
				hasPrevLastUnstableI = false
			}

			advanceChan = nil
		}
	}
}

func (n *Node) Tick() {
	select {
	case n.tickChan <- struct{}{}:
	default:
	}
}

func (n *Node) Campaign(ctx context.Context) error {
	return n.step(ctx, Message{Type: MsgHup})
}

func (n *Node) Propose(ctx context.Context, data []byte) error {
	return n.step(ctx, Message{Type: MsgProp, Entries: []Entry{{Data: data}}})
}

func (n *Node) ProposeConfChange(ctx context.Context, cc ConfChange) error {
	data, _ := json.Marshal(cc)

	return n.step(ctx, Message{Type: MsgProp, Entries: []Entry{{Data: data}}})
}

func (n *Node) Ready() <-chan Ready {
	return n.readyChan
}

func (n *Node) Advance() {
	n.advanceChan <- struct{}{}
}

func (n *Node) step(ctx context.Context, m Message) error {
	ch := n.recvChan
	if m.Type == MsgProp {
		ch = n.proc
	}

	select {
	case ch <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
