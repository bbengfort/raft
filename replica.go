package raft

import "github.com/bbengfort/x/peers"

// Replica represents the local consensus replica and is the primary object
// in the system. There should only be one replica per process (and many peers).
// TODO: document more.
type Replica struct {
	peers.Peer

	// Network Defintion
	config  *Config            // configuration values
	remotes map[string]*Remote // remote peers on the network

	// Consensus State
	state    State     // the current behavior of the local replica
	leader   string    // the name of the leader of the quorum
	term     uint64    // current term of the replica
	log      *Log      // state machine command log maintained by consensus
	votes    *Election // the current leader election, if any
	votedFor string    // the peer we voted for in the current term
	ticker   *Ticker   // emits timing events
}

func (r *Replica) Listen() error        { return nil }
func (r *Replica) Close() error         { return nil }
func (r *Replica) Dispatch(Event) error { return nil }
func (r *Replica) Handle(Event) error   { return nil }
