package raft

import (
	"fmt"
	"time"

	"github.com/bbengfort/raft/pb"
)

// NewLog creates and initializes a new log whose first entry is the NullEntry.
func NewLog(sm StateMachine) *Log {
	entries := make([]*pb.LogEntry, 1)
	entries[0] = pb.NullEntry

	return &Log{
		sm:      sm,
		entries: entries,
		created: time.Now(),
		updated: time.Now(),
	}
}

// Log implements the sequence of commands applied to the Raft state machine.
// This implementation uses an entirely in-memory log that snapshots to disk
// occassionally for durability. The log ensures that the sequence of commands
// is consistent, e.g. that entries are appendended in monotonically increasing
// time order as defined by the Raft leader's term.
//
// Logs generate two types of events: entry committed and entry dropped. Commit
// events are dispatched in the order of the log, so the they are seen
// sequentially in order to apply them to the state machine. Dropped events
// occur when a log's uncommitted entries are truncated in response to
// leadership changes, these events also occur in order, though they have no
// impact on the state machine itself.
//
// Note that the log is not thread-safe, and is not intended to be accessed
// from multiple go routines. Instead the log should be maintained by a single
// state machine that updates it sequentially when events occur.
type Log struct {
	sm          StateMachine   // The state machine to dispatch events for
	lastApplied uint64         // The index of the last applied log entry
	commitIndex uint64         // The index of the last committed log entry
	entries     []*pb.LogEntry // In-memory array of log entries
	created     time.Time      // Timestamp the log was created
	updated     time.Time      // Timestamp of the last log modification
}

//===========================================================================
// Index Management
//===========================================================================

// LastApplied returns the index of the last applied log entry.
func (l *Log) LastApplied() uint64 {
	return l.lastApplied
}

// CommitIndex returns the index of the last committed log entry.
func (l *Log) CommitIndex() uint64 {
	return l.commitIndex
}

// LastEntry returns the log entry at the last applied index.
func (l *Log) LastEntry() *pb.LogEntry {
	return l.entries[l.lastApplied]
}

// LastCommit returns the log entry at the commit index.
func (l *Log) LastCommit() *pb.LogEntry {
	return l.entries[l.commitIndex]
}

// LastTerm is a helper function to get the term of the entry at the last applied index.
func (l *Log) LastTerm() uint64 {
	return l.LastEntry().Term
}

// CommitTerm is a helper function to get the term of the entry at the commit index.
func (l *Log) CommitTerm() uint64 {
	return l.LastCommit().Term
}

// AsUpToDate returns true if the remote log specified by the last index and
// last term are at least as up to date (or farther ahead) than the local log.
func (l *Log) AsUpToDate(lastIndex, lastTerm uint64) bool {
	localTerm := l.LastTerm()

	// If we're in the same term as the remote host, our last applied index
	// should be at least as large as the remote's last applied index.
	if lastTerm == localTerm {
		return lastIndex >= l.lastApplied
	}

	// Otherwise ensure that the remote's term is greater than our own.
	return lastTerm > localTerm
}

//===========================================================================
// Entry Management
//===========================================================================

// Create an entry in the log and append it. This is essentially a helper method
// for quickly adding a command to the state machine consistent with the local log.
func (l *Log) Create(name string, value []byte, term uint64) (*pb.LogEntry, error) {

	// Create the entry at the next log index
	entry := &pb.LogEntry{
		Index: l.lastApplied + 1,
		Term:  term,
		Name:  name,
		Value: value,
	}

	// Append the entry and perform invariant checks
	if err := l.Append(entry); err != nil {
		return nil, err
	}

	// Return the entry for use elsewhere
	return entry, nil
}

// Append one ore more entries and perform log invariant checks. If appending
// an entry creates a log inconsistency (out of order term or index), then an
// error is returned. A couple of important notes:
//
//  1. Append does not undo any successful appends even on error
//  2. Append will not compare entries that specify the same index
//
// These notes mean that all entries being appended to this log should be
// consistent with each other as well as the end of the log, and that the log
// needs to be truncated in order to "update" or splice two logs together.
func (l *Log) Append(entries ...*pb.LogEntry) error {
	// Append all entries one at a time, returning an error if an append fails.
	for _, entry := range entries {

		// Fetch the latest entry
		prev := l.LastEntry()

		// Ensure that the term is monotonically increasing
		if entry.Term < prev.Term {
			return fmt.Errorf("cannot append entry in earlier term (%d < %d)", entry.Term, prev.Term)
		}

		// Ensure that the index is monotonically increasing
		if entry.Index <= prev.Index {
			return fmt.Errorf("cannot append entry with smaller index (%d <= %d)", entry.Index, prev.Index)
		}

		// Append the entry and update metadata
		l.entries = append(l.entries, entry)
		l.lastApplied++

	}

	return nil
}

// Commit all entries up to and including the specified index.
func (l *Log) Commit(index uint64) error {
	// Ensure the index specified is in the log
	if index < 1 || index > l.lastApplied {
		return fmt.Errorf("cannot commit invalid index %d", index)
	}

	// Ensure that we haven't already committed this index
	if index <= l.commitIndex {
		return fmt.Errorf("index at %d already committed", index)
	}

	// Create a commit event for all entries now committed
	for i := l.commitIndex + 1; i <= index; i++ {
		if err := l.sm.CommitEntry(l.entries[i]); err != nil {
			return err
		}
	}

	// Update the commit index and the log
	l.commitIndex = index
	debug("committed index %d", l.commitIndex)
	return nil
}

// Truncate the log to the given position, conditioned by term.
// This method freturns an error if the log has been committed after the
// specified index, there is an epoch mismatch, or there is some other log
// operation error.
//
// This method truncates everything after the given index, but keeps the
// entry at the specified index; e.g. truncate after.
func (l *Log) Truncate(index, term uint64) error {
	// Ensure the truncation matches an entry
	if index > l.lastApplied {
		return fmt.Errorf("cannot truncate invalid index %d", index)
	}

	// Specifies the index of the entry to be truncated
	nextIndex := index + 1

	// Do not allow committed entries to be truncted
	if nextIndex <= l.commitIndex {
		return fmt.Errorf("cannot truncate already committed index %d", index)
	}

	// Do not truncate if entry at index does not have matching term
	entry := l.entries[index]
	if entry.Term != term {
		return fmt.Errorf("entry at index %d does not match term %d", index, term)
	}

	// Only perform truncation if necessary
	if index < l.lastApplied {
		// Drop all entries that appear after the index
		for _, droppedEntry := range l.entries[nextIndex:] {
			if err := l.sm.DropEntry(droppedEntry); err != nil {
				return err
			}
		}

		// Update the entries and meta data
		l.entries = l.entries[0:nextIndex]
		l.lastApplied = index
	}

	return nil
}

//===========================================================================
// Entry Access
//===========================================================================

// Get the entry at the specified index (whether or not it is committed).
// Returns an error if no entry exists at the index.
func (l *Log) Get(index uint64) (*pb.LogEntry, error) {
	if index > l.lastApplied {
		return nil, fmt.Errorf("no entry at index %d", index)
	}

	return l.entries[index], nil
}

// Prev returns the entry before the specified index (whether or not it is
// committed). Returns an error if no entry exists before.
func (l *Log) Prev(index uint64) (*pb.LogEntry, error) {
	if index < 1 || index > (l.lastApplied+1) {
		return nil, fmt.Errorf("no entry before index %d", index)
	}

	return l.entries[index-1], nil
}

// After returns all entries after the specified index, inclusive
func (l *Log) After(index uint64) ([]*pb.LogEntry, error) {
	if index > l.lastApplied {
		return make([]*pb.LogEntry, 0), fmt.Errorf("no entries after %d", index)
	}

	return l.entries[index:], nil
}
