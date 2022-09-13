package raft

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
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
		log.Debug().Str("replica", r.Name).Msg("already stopped")
		return nil
	}

	r.ticker.StopAll()
	log.Debug().Str("replica", r.Name).Msg("replica has been stopped")
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

	log.Debug().Str("replica", r.Name).Msg("replica has been initialized")
	return nil
}

// Should only be called once after initialization to bootstrap the quorum by
// starting the leader's heartbeat or starting the election timeout for all
// other replicas.
func (r *Replica) setRunningState() error {
	if r.state != Initialized {
		return errors.New("can only set running state from initialized")
	}

	// Start election timeout
	r.ticker.Start(ElectionTimeout)

	if r.config.IsLeader() {
		// Bootstrap leader by starting election
		go r.Dispatch(&event{etype: ElectionTimeout, source: nil, value: nil})
	}

	log.Debug().Str("replica", r.Name).Msg("replica is now running")
	return nil
}

func (r *Replica) setFollowerState() error {
	// Reset volatile state
	r.setInitializedState()

	// Update the tickers for following
	r.ticker.Stop(HeartbeatTimeout)
	r.ticker.Start(ElectionTimeout)

	log.Info().Str("replica", r.Name).Uint64("term", r.term).Msg("replica is now a follower")
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

	log.Info().Str("replica", r.Name).Uint64("term", r.term).Msg("replica is now a candidate")
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
	log.Info().Str("replica", r.Name).Uint64("term", r.term).Msg("replica is now the leader")
	return nil
}
