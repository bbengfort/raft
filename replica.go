package raft

import (
	"fmt"
	"net"

	pb "github.com/bbengfort/raft/api/v1beta1"
	"github.com/bbengfort/x/peers"
	"google.golang.org/grpc"
)

// Replica represents the local consensus replica and is the primary object
// in the system. There should only be one replica per process (and many peers).
// TODO: document more.
type Replica struct {
	pb.UnimplementedRaftServer
	peers.Peer

	// TODO: remove when stable
	Metrics *Metrics // keep track of access statistics

	// Network Defintion
	config  *Config                         // configuration values
	events  chan Event                      // event handler channel
	remotes map[string]*Remote              // remote peers on the network
	clients map[uint64]chan *pb.CommitReply // Respond to clients on commit/drop

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
	addr := fmt.Sprintf(":%d", r.Port)
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
	if r.config.Aggregate {
		if err := r.runAggregatingEventLoop(); err != nil {
			return err
		}
	} else {
		if err := r.runEventLoop(); err != nil {
			return err
		}
	}

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
	case CommitRequestEvent:
		return r.onCommitRequest(e)
	case AggregatedCommitRequestEvent:
		return r.onAggregatedCommitRequest(e)
	case VoteRequestEvent:
		return r.onVoteRequest(e)
	case VoteReplyEvent:
		return r.onVoteReply(e)
	case AppendRequestEvent:
		return r.onAppendRequest(e)
	case AppendReplyEvent:
		return r.onAppendReply(e)
	case ErrorEvent:
		return e.Value().(error)
	case CommitEvent:
		return nil
	case DropEvent:
		return nil
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

// CommitEntry responds to the client with a successful entry commit.
func (r *Replica) CommitEntry(entry *pb.LogEntry) error {
	debug("commit entry %d in term %d: applying %s", entry.Index, entry.Term, entry.Name)

	client, ok := r.clients[entry.Index]
	if !ok {
		// Entry committed at follower that is not responding to clients.
		return nil
	}

	client <- &pb.CommitReply{
		Success: true, Entry: entry, Redirect: "", Error: "",
	}

	// Record successful response
	go func() { r.Metrics.Complete(true) }()

	// Ensure the map is cleaned up after response!
	delete(r.clients, entry.Index)
	return nil
}

// DropEntry responds to the client of an unsuccessful commit.
func (r *Replica) DropEntry(entry *pb.LogEntry) error {
	debug("drop entry %d in term %d", entry.Index, entry.Term)

	client, ok := r.clients[entry.Index]
	if !ok {
		// Entry committed at follower that is not responding to clients.
		return nil
	}

	err := fmt.Sprintf("entry could not be committed in term %d", entry.Term)
	client <- &pb.CommitReply{
		Success: false, Entry: nil, Redirect: "", Error: err,
	}

	// Record dropped response
	go func() { r.Metrics.Complete(false) }()

	// Ensure the map is cleaned up after response!
	delete(r.clients, entry.Index)
	return nil
}

//===========================================================================
// Event Loops
//===========================================================================

// Runs a normal event loop, handling one event at a time.
func (r *Replica) runEventLoop() error {
	defer func() {
		// nilify the events channel when we stop running it
		r.events = nil
	}()

	for e := range r.events {
		if err := r.Handle(e); err != nil {
			return err
		}
	}

	return nil
}

// Runs an event loop that aggregates multiple commit requests into a single
// append entries request that is sent to peers at once. this optimizes the
// benchmarking case and improves response times to clients during high volume
// periods. This is the primary addition for 0.4 functionality.
func (r *Replica) runAggregatingEventLoop() error {
	defer func() {
		// nilify the events channel when we stop running it
		r.events = nil
	}()

	for e := range r.events {
		if e.Type() == CommitRequestEvent {
			// If we have a commit request, attempt to aggregate, keeping
			// track of a next value (defaulting to nil) and storing all
			// commit requests in an array to be handled at once.
			var next Event
			requests := []Event{e}

		aggregator:
			// The aggregator for loop keeps reading events off the channel
			// until there is nothing on it, or a non-commit request event is
			// read. In the meantime it aggregates all commit requests on the
			// event channel into a single events array. Note the non-blocking
			// read via the select with a default case.
			for {
				select {
				case next = <-r.events:
					if next.Type() != CommitRequestEvent {
						// exit aggregator loop and handle next and requests
						break aggregator
					} else {
						// continue to aggregate commit request events
						requests = append(requests, next)
					}
				default:
					// nothing is on the channel, break aggregator and do not
					// handle the empty next value by marking it as nil
					next = nil
					break aggregator
				}
			}

			// This section happens after the aggregator for loop is complete
			// First handle the commit request events, using an aggregated event
			// if more than one request was found, otherwise handling normally.
			if len(requests) > 1 {
				ae := &event{etype: AggregatedCommitRequestEvent, source: nil, value: requests}
				if err := r.Handle(ae); err != nil {
					return err
				}
			} else {
				// Handle the single commit request without the aggregator
				// TODO: is this necessary?
				if err := r.Handle(requests[0]); err != nil {
					return err
				}
			}

			// Second, handle the next event if one exists
			if next != nil {
				if err := r.Handle(next); err != nil {
					return err
				}
			}

		} else {
			// Otherwise handle event normally without aggregation
			if err := r.Handle(e); err != nil {
				return err
			}
		}
	}

	return nil
}
