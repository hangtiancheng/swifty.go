package raft

import (
	"errors"
	"sync"
)

var ErrCompacted = errors.New("requested index is unavailable due to compaction")

var ErrUnavailable = errors.New("requested entry at index is unavailable")

type Storage interface {
	InitialState() (HardState, ConfState, error)
	// Append persists the given entries, truncating any conflicting tail.
	Append(entries []Entry) error
	// SetHardState persists the given hard state.
	SetHardState(hardState HardState) error
	// Entries returns the entries in the range [l, r).
	Entries(l, r uint64) ([]Entry, error)
	// Term returns the term of the entry at the given index.
	Term(i uint64) (uint64, error)
	// LastIndex returns the index of the last persisted entry.
	LastIndex() (uint64, error)
	// FirstIndex returns the index of the first persisted entry.
	FirstIndex() (uint64, error)
}

type MemoryStorage struct {
	sync.Mutex
	hardState HardState
	// Persisted entries; ents[0] is a dummy entry holding the offset.
	ents []Entry
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		ents: make([]Entry, 1),
	}
}

func (m *MemoryStorage) InitialState() (HardState, ConfState, error) {
	return m.hardState, ConfState{}, nil
}

// SetHardState persists the given hard state.
func (m *MemoryStorage) SetHardState(hardState HardState) error {
	m.Lock()
	defer m.Unlock()
	m.hardState = hardState
	return nil
}

// Append persists the given entries, truncating any conflicting tail first.
func (m *MemoryStorage) Append(entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	m.Lock()
	defer m.Unlock()

	offset := m.ents[0].Index
	first := entries[0].Index
	if first < offset {
		return ErrCompacted
	}

	// Drop the local tail that conflicts with the incoming entries.
	if last := m.lastIndex(); first <= last {
		m.ents = m.ents[:first-offset]
	} else if first > last+1 {
		return ErrUnavailable
	}
	m.ents = append(m.ents, entries...)
	return nil
}

func (m *MemoryStorage) Entries(l, r uint64) ([]Entry, error) {
	m.Lock()
	defer m.Unlock()

	offset := m.ents[0].Index
	if l <= offset {
		return nil, ErrCompacted
	}

	if r > m.lastIndex()+1 {
		return nil, ErrUnavailable
	}

	if len(m.ents) == 1 {
		return nil, ErrUnavailable
	}

	return m.ents[l-offset : r-offset], nil

}

func (m *MemoryStorage) Term(i uint64) (uint64, error) {
	m.Lock()
	defer m.Unlock()
	offset := m.ents[0].Index
	if i < offset {
		return 0, ErrCompacted
	}

	if int(i-offset) >= len(m.ents) {
		return 0, ErrUnavailable
	}

	return m.ents[i-offset].Term, nil
}

func (m *MemoryStorage) LastIndex() (uint64, error) {
	m.Lock()
	defer m.Unlock()
	return m.lastIndex(), nil
}

func (m *MemoryStorage) lastIndex() uint64 {
	return m.ents[0].Index + uint64(len(m.ents)) - 1
}

func (m *MemoryStorage) FirstIndex() (uint64, error) {
	m.Lock()
	defer m.Unlock()
	return m.firstIndex(), nil
}

func (m *MemoryStorage) firstIndex() uint64 {
	return m.ents[0].Index + 1
}
