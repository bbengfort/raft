package raft

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
