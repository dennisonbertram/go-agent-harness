package main

import (
	"sync"
)

type State int

const (
	Follower State = iota
	Candidate
	Leader
)

type LogEntry struct {
	Term    int
	Command string
}

type RaftNode struct {
	mu          sync.Mutex
	state       State
	log         []LogEntry
	commitIndex int
	currentTerm int
	votedFor    int
}

func NewRaftNode() *RaftNode {
	return &RaftNode{
		state:    Follower,
		votedFor: -1,
	}
}

func (r *RaftNode) AppendEntries(term int, entries []LogEntry) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if term < r.currentTerm {
		return false
	}
	if term > r.currentTerm {
		r.currentTerm = term
		r.state = Follower
		r.votedFor = -1
	}
	r.log = append(r.log, entries...)
	if len(r.log) > 0 {
		r.commitIndex = len(r.log) - 1
	}
	return true
}

func (r *RaftNode) RequestVote(term int, candidateID int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if term < r.currentTerm {
		return false
	}
	if term > r.currentTerm {
		r.currentTerm = term
		r.votedFor = -1
	}
	if r.votedFor == -1 || r.votedFor == candidateID {
		r.votedFor = candidateID
		return true
	}
	return false
}

func (r *RaftNode) Propose(cmd string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != Leader {
		return -1, ErrNotLeader
	}

	entry := LogEntry{Term: r.currentTerm, Command: cmd}
	r.log = append(r.log, entry)
	r.commitIndex = len(r.log) - 1
	return r.commitIndex, nil
}

func (r *RaftNode) stepDown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = Follower
}

var ErrNotLeader = &RaftError{"not leader"}

type RaftError struct {
	msg string
}

func (e *RaftError) Error() string {
	return e.msg
}
