package raft

//===========================================================================
// Election Helpers
//===========================================================================

// NewElection creates an election for the specified peers, defaulting the
// votes to false until otherwise updated.
func NewElection(peers ...string) *Election {
	votes := new(Election)
	votes.ballots = make(map[string]bool, len(peers))
	for _, name := range peers {
		votes.ballots[name] = false
	}
	return votes
}

// Election objects keep track of the outcome of a single leader election by
// mapping remote peers to the votes they've provided. Uses simple majority
// to determine if an election has passed or failed.
type Election struct {
	ballots map[string]bool // The votes cast during the election (yes/no)
}

// Vote records the vote for the given Replica, identified by name.
func (e *Election) Vote(name string, vote bool) {
	e.ballots[name] = vote
}

// Majority computes how many nodes are needed for an election to pass.
func (e *Election) Majority() int {
	return (len(e.ballots) / 2) + 1
}

// Votes sums the number of Ballots equal to true
func (e *Election) Votes() int {
	count := 0
	for _, ballot := range e.ballots {
		if ballot {
			count++
		}
	}
	return count
}

// Passed returns true if the number of True votes is a majority.
func (e *Election) Passed() bool {
	return e.Votes() >= e.Majority()
}
