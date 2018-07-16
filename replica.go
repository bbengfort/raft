package raft

import (
	"fmt"
	"net"

	"github.com/bbengfort/raft/pb"
	"github.com/bbengfort/x/peers"
	"google.golang.org/grpc"
)

// Replica represents the local consensus replica and is the primary object
// in the system. There should only be one replica per process (and many peers).
// TODO: document more.
type Replica struct {
	peers.Peer

	// Network Defintion
	config  *Config            // configuration values
	events  chan Event         // event handler channel
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

// Listen for messages from peers and clients and run the event loop.
func (r *Replica) Listen() error {
	// Open TCP socket to listen for messages
	addr := r.Endpoint(false)
	sock, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s", addr)
	}
	defer sock.Close()
	info("listening for requests on %s", addr)

	// Initialize and run the gRPC server in its own thread
	srv := grpc.NewServer()
	pb.RegisterRaftServer(srv, r)
	go srv.Serve(sock)

	// Create the events channel and set the state to running
	r.events = make(chan Event, actorEventBufferSize)
	r.setState(Running)

	// Run event handling loop
	for event := range r.events {
		if err := r.Handle(event); err != nil {
			return err
		}
	}

	// If the events channel has been exhausted, nilify it
	r.events = nil
	return nil
}

// Close the event handler and stop listening for events.
func (r *Replica) Close() error {
	if r.events == nil {
		return ErrNotListening
	}

	close(r.events)
	return nil
}

// Dispatch events by clients to the replica.
func (r *Replica) Dispatch(e Event) error {
	if r.events == nil {
		return ErrNotListening
	}

	r.events <- e
	return nil
}

// Handle the events in serial order.
func (r *Replica) Handle(e Event) error {
	trace("%s event received: %v", e.Type(), e.Value())

	switch e.Type() {
	case HeartbeatTimeout:
		return r.onHeartbeatTimeout(e)
	case ElectionTimeout:
		return r.onElectionTimeout(e)
	case VoteRequestEvent:
		return r.onVoteRequest(e)
	case VoteReplyEvent:
		return r.onVoteReply(e)
	case AppendRequestEvent:
		return r.onAppendRequest(e)
	case AppendReplyEvent:
		return r.onAppendReply(e)
	default:
		return fmt.Errorf("no handler identified for event %s", e.Type())
	}
}

// Quorum creates a new election with all configured peers.
func (r *Replica) Quorum() *Election {
	peers := make([]string, 0, len(r.config.Peers))
	for _, peer := range r.config.Peers {
		peers = append(peers, peer.Name)
	}
	return NewElection(peers...)
}

// CheckRPCTerm ensures that the replica is in the correct state relative to
// the term of a remote replica. If the term from the remote is larger than
// local term, we update our term and set our state to follower.
func (r *Replica) CheckRPCTerm(term uint64) (updated bool, err error) {
	if term > r.term {
		r.term = term
		err = r.setState(Follower)
		return true, err
	}
	return false, nil
}

// CheckCommits works backward from the last applied index to the commit index
// checking to see if a majority of peers matches that index, and if so,
// committing all entries prior to the match index.
func (r *Replica) CheckCommits() error {
	for n := r.log.lastApplied; n > r.log.commitIndex; n-- {

		// Create a quorum for voting for the commit index
		commit := r.Quorum()
		commit.Vote(r.Name, true)

		// Vote peer's match index
		for _, peer := range r.remotes {
			commit.Vote(peer.Name, peer.matchIndex >= n)
		}

		// Commit the index if its term matches our term
		if commit.Passed() && r.log.entries[n].Term == r.term {
			return r.log.Commit(n)
		}

	}

	return nil
}
