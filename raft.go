/*
Package raft implements the Raft consensus algorithm.
*/
package raft

// Replica represents the local consensus replica and is the primary object
// in the system. There should only be one replica per process (and many peers).
// TODO: document more.
type Replica struct {

	// Consensus State
	state    State     // the current behavior of the local replica
	leader   string    // the name of the leader of the quorum
	term     uint64    // current term of the replica
	log      *Log      // state machine command log maintained by consensus
	votes    *Election // the current leader election, if any
	votedFor string    // the peer we voted for in the current term
	ticker   *Ticker   // emits timing events
}
