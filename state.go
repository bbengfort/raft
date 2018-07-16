package raft

import (
	"errors"
	"fmt"
)

// Raft server states (not part of the state machine)
const (
	Stopped State = iota // stopped should be the zero value and default
	Initialized
	Running
	Follower
	Candidate
	Leader
)

// Names of the states for serialization
var stateStrings = [...]string{
	"stopped", "initialized", "running", "follower", "candidate", "leader",
}

//===========================================================================
// State Enumeration
//===========================================================================

// State is an enumeration of the possible status of a replica.
type State uint8

// String returns a human readable representation of the state.
func (s State) String() string {
	return stateStrings[s]
}

//===========================================================================
// State Transitions
//===========================================================================

// SetState updates the state of the local replica, performing any actions
// related to multiple states, modifying internal private variables as
// needed and calling the correct internal state setting function.
//
// NOTE: These methods are not thread-safe.
func (r *Replica) setState(state State) (err error) {
	switch state {
	case Stopped:
		err = r.setStoppedState()
	case Initialized:
		err = r.setInitializedState()
	case Running:
		err = r.setRunningState()
	case Follower:
		err = r.setFollowerState()
	case Candidate:
		err = r.setCandidateState()
	case Leader:
		err = r.setLeaderState()
	default:
		err = fmt.Errorf("unknown state '%s'", state)
	}

	if err == nil {
		r.state = state
	}

	return err
}

// Stops all timers that might be running.
func (r *Replica) setStoppedState() error {
	if r.state == Stopped {
		debug("%s is already stopped", r.Name)
		return nil
	}

	r.ticker.StopAll()

	info("%s has been stopped", r.Name)
	return nil
}

// Resets any volatile variables on the local replica and is called when the
// replica becomes a follower or a candidate.
func (r *Replica) setInitializedState() error {
	r.votes = nil
	r.votedFor = ""

	// TODO: reset remote peers configuration.
	for _, peer := range r.remotes {
		peer.nextIndex = 0
		peer.matchIndex = 0
	}

	debug("%s has been initialized", r.Name)
	return nil
}

// Should only be called once after initialization to bootstrap the quorum by
// starting the leader's heartbeat or starting the election timeout for all
// other replicas.
func (r *Replica) setRunningState() error {
	if r.state != Initialized {
		return errors.New("can only set running state from initialized")
	}

	// TODO: add bootstrap code
	// Start election timeout to elect the leader if None
	r.ticker.Start(ElectionTimeout)

	status("%s is now running", r.Name)
	return nil
}

func (r *Replica) setFollowerState() error {
	// Reset volatile state
	r.setInitializedState()

	// Update the tickers for following
	r.ticker.Stop(HeartbeatTimeout)
	r.ticker.Start(ElectionTimeout)

	status("%s is now a follower", r.Name)
	return nil
}

func (r *Replica) setCandidateState() error {
	// Reset volatile state
	r.setInitializedState()

	// Create the election for the next term and vote for self
	r.term++
	r.votes = r.Quorum()
	r.votes.Vote(r.Name, true)

	// Notify all replicas of the vote request
	lastLogIndex := r.log.LastApplied()
	lastLogTerm := r.log.LastTerm()
	for _, remote := range r.remotes {
		if err := remote.RequestVote(r.Name, r.term, lastLogIndex, lastLogTerm); err != nil {
			return err
		}
	}

	status("%s is candidate for term %d", r.Name, r.term)
	return nil
}

func (r *Replica) setLeaderState() error {
	if r.state == Leader {
		return nil
	}

	// Stop the election timeout if we're leader
	r.ticker.Stop(ElectionTimeout)
	r.leader = r.Name

	// Set the volatile state for known state of followers
	for _, peer := range r.remotes {
		peer.nextIndex = r.log.LastApplied() + 1
		peer.matchIndex = 0
	}

	// Broadcast the initial heartbeat AppendEntries message
	for _, remote := range r.remotes {
		if err := remote.AppendEntries(r.leader, r.term, r.log); err != nil {
			return err
		}
	}

	// Start the heartbeat interval
	r.ticker.Start(HeartbeatTimeout)
	status("%s is leader for term %d", r.Name, r.term)
	return nil
}
