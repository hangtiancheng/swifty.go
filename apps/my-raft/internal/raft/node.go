package raft

import (
	"context"
	"encoding/json"
)

type Node struct {
	// Channel for locally submitted data.
	proc chan Message
	// Channel for data submitted by other nodes.
	recvc chan Message
	// Channel for configuration change requests.
	confc chan Message
	// Channel that delivers the results of Raft processing.
	readyc chan Ready
	// Channel that advances the algorithm after a Ready has been consumed.
	advancec chan struct{}
	// Timer channel.
	tickc chan struct{}
}

// StartNode creates a Raft node from the given configuration and peer list.
func StartNode(conf *Config, peers []Peer) Node {
	r := newRaft(conf)
	// Add all peers to the configuration.
	// for _, peer := range peers {

	// }
	r.raftLog.commitIndex = r.raftLog.lastIndex()

	// Add the per-node progress information.
	// for _, peer := range peers {
	// 	r.addNode(peer.ID)
	// }

	n := newNode()
	go n.run(r)
	return n
}

func newNode() Node {
	return Node{
		proc:     make(chan Message),
		recvc:    make(chan Message),
		confc:    make(chan Message),
		readyc:   make(chan Ready),
		advancec: make(chan struct{}),
		tickc:    make(chan struct{}),
	}
}

func (n *Node) run(r *raft) {
	var (
		readyc   chan Ready
		advancec chan struct{}
		// State used to detect whether there is anything new to publish.
		rd       Ready
		prevSoft = r.softState()
		prevHard = emptyHardState

		prevLastUnstablei, prevLastUnstablet uint64
		hasPrevLastUnstablei                 bool
	)

	for {
		// If we are waiting for an Advance, nothing can be published yet.
		if advancec != nil {
			readyc = nil
			// Otherwise build a new Ready and deliver it on readyc whenever
			// it contains any updates.
		} else if rd = newReady(r, prevSoft, prevHard); rd.containsUpdates() {
			readyc = n.readyc
		} else {
			readyc = nil
		}

		select {
		// A locally submitted message.
		case m := <-n.proc:
			// Handle the local proposal.
			m.From = r.id
			r.Step(m)
		case m := <-n.recvc:
			// Ignore messages from senders that are not part of the cluster,
			// except for non-response messages such as vote requests.
			if _, ok := r.prs[m.From]; ok || !IsResponseMsg(m.Type) {
				r.Step(m)
			}
		case <-n.confc:
			// Configuration change request (not wired up yet).
			// The timer fired.
		case <-n.tickc:
			r.tick()
			// There is something to publish.
		case readyc <- rd:
			// Sync the state that has been published, so that the next loop
			// iteration can tell whether there is anything new.
			if rd.SoftState != nil {
				prevSoft = rd.SoftState
			}

			if !IsEmptyHardState(rd.HardState) {
				prevHard = rd.HardState
			}

			if len(rd.Entries) > 0 {
				prevLastUnstablei, prevLastUnstablet = rd.Entries[len(rd.Entries)-1].Index, rd.Entries[len(rd.Entries)-1].Term
				hasPrevLastUnstablei = true
			}

			r.msgs = nil
			r.readStates = nil
			advancec = n.advancec
		case <-advancec:
			// The previous Ready is considered persisted and applied, because
			// the application only calls Advance after consuming it.
			if prevHard.CommitIndex != 0 {
				r.raftLog.appliedTo(prevHard.CommitIndex)
			}

			if hasPrevLastUnstablei {
				r.raftLog.stableTo(prevLastUnstablei, prevLastUnstablet)
				hasPrevLastUnstablei = false
			}

			advancec = nil
		}
	}
}

// Tick delivers a tick to the Raft node. The tick is dropped if the previous
// one has not been consumed yet.
func (n *Node) Tick() {
	select {
	case n.tickc <- struct{}{}:
	default:
	}
}

// Campaign starts an election for this node.
func (n *Node) Campaign(ctx context.Context) error {
	return n.step(ctx, Message{Type: MsgHup})
}

// Propose submits data to be replicated through the Raft cluster.
func (n *Node) Propose(ctx context.Context, data []byte) error {
	return n.step(ctx, Message{Type: MsgProp, Entries: []Entry{{Data: data}}})
}

// ProposeConfChange submits a configuration change to the cluster.
func (n *Node) ProposeConfChange(ctx context.Context, cc ConfChange) error {
	data, _ := json.Marshal(cc)

	return n.step(ctx, Message{Type: MsgProp, Entries: []Entry{{Data: data}}})
}

// Ready returns the channel on which the node publishes updates that the
// application must persist, send, and apply.
func (n *Node) Ready() <-chan Ready {
	return n.readyc
}

// Advance notifies the node that the application has finished consuming the
// most recent Ready.
func (n *Node) Advance() {
	n.advancec <- struct{}{}
}

func (n *Node) step(ctx context.Context, m Message) error {
	ch := n.recvc
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
