package raft

type Ready struct {
	// SoftState is the volatile state.
	SoftState *SoftState

	// HardState is the state that must be persisted.
	HardState HardState

	// ReadStates are the results of linearizable read requests.
	ReadStates []ReadState

	// Entries must be persisted before any messages are sent.
	Entries []Entry

	// CommittedEntries are entries that are committed and ready to be
	// applied to the state machine.
	CommittedEntries []Entry

	// Message is the list of messages to send.
	Message []Message
}

func newReady(r *raft, preSoft *SoftState, preHard HardState) Ready {
	rd := Ready{}
	// Copy the entry slices: the application may hold the Ready while the log
	// keeps appending to and re-slicing its own arrays.
	if ents := r.raftLog.unstableEntries(); len(ents) > 0 {
		rd.Entries = append([]Entry{}, ents...)
	}
	if ents := r.raftLog.nextEnts(); len(ents) > 0 {
		rd.CommittedEntries = append([]Entry{}, ents...)
	}
	// Messages waiting to be sent.
	rd.Message = r.msgs
	if soft := r.softState(); !soft.equal(preSoft) {
		rd.SoftState = soft
	}
	if hard := r.hardState(); !isHardStateEqual(hard, preHard) {
		rd.HardState = hard
	}
	if len(r.readStates) != 0 {
		rd.ReadStates = r.readStates
	}
	return rd
}

func (rd Ready) containsUpdates() bool {
	return rd.SoftState != nil || !IsEmptyHardState(rd.HardState) ||
		len(rd.Entries) > 0 || len(rd.CommittedEntries) > 0 ||
		len(rd.Message) > 0 || len(rd.ReadStates) > 0
}

// IsEmptyHardState returns true if the given hard state is empty.
func IsEmptyHardState(h HardState) bool {
	return isHardStateEqual(h, emptyHardState)
}
