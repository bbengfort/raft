package raft

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/bbengfort/raft/api/v1beta1"
	"github.com/bbengfort/x/peers"
	out "github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Remote maintains a connection to a peer on the network.
type Remote struct {
	peers.Peer

	actor      Actor                       // the listener to dispatch events to
	timeout    time.Duration               // timeout before dropping message
	conn       *grpc.ClientConn            // grpc dial connection to the remote
	client     pb.RaftClient               // rpc client specified by protobuf
	stream     pb.Raft_AppendEntriesClient // Append Entries stream
	online     bool                        // if the client is connected or not
	nextIndex  uint64                      // local state of the remote's next log index
	matchIndex uint64                      // local state of the remote's commit index
}

// NewRemote creates a new remote associated with the replica.
func NewRemote(p peers.Peer, r *Replica) (*Remote, error) {
	timeout, err := r.config.GetTimeout()
	if err != nil {
		return nil, err
	}

	remote := &Remote{Peer: p, actor: r, timeout: timeout}
	return remote, nil
}

//===========================================================================
// RPC Wrappers
//===========================================================================

// RequestVote from other members of the quorum. This method initiates a go
// routine to send the vote and will put any response onto the event queue.
// Send errors are ignored as the connection will simply be put into offline
// mode, and retries can be made in the next election.
//
// Dispatches VoteReplyEvents.
func (c *Remote) RequestVote(candidate string, term, lastLogIndex, lastLogTerm uint64) error {
	go func() {
		req := &pb.VoteRequest{
			Term:         term,
			Candidate:    candidate,
			LastLogIndex: lastLogIndex,
			LastLogTerm:  lastLogTerm,
		}

		rep, err := c.send(func(ctx context.Context) (interface{}, error) {
			return c.client.RequestVote(ctx, req)
		})

		if err != nil {
			// errors are not fatal, just go offline
			return
		}

		// Dispatch the vote reply event
		c.actor.Dispatch(&event{
			etype:  VoteReplyEvent,
			source: c,
			value:  rep,
		})

	}()

	return nil
}

// AppendEntries from leader to followers in quorum; this acts as a heartbeat
// as well as the primary consensus mechanism. The method requires access to
// the log, since the remote stores the state of the remote log. In order to
// ensure consistency, log accesses happen synchronously, then the method
// initiates a go routine to send the RPC asynchronously and dispatches an
// event on reply. Send errors are ignored as the connection will simply be
// put into offline mode, and retries can be made after the next heartbeat.
//
// Dispatches AppendReplyEvents.
func (c *Remote) AppendEntries(leader string, term uint64, log *Log) error {
	// NOTE: access the log synchronously before dispatching the go routine.
	commitIndex := log.CommitIndex()

	// If there are no entries after the next index, ignore the error and
	// use the empty slice that after returns to send no entries.
	entries, _ := log.After(c.nextIndex)

	// If there is no previous entry, a fatal error has occurred.
	prev, err := log.Prev(c.nextIndex)
	if err != nil {
		return err
	}

	// Now that we've read the log state, initiate the request in its own thread
	go func() {

		// Log the append entries message
		if len(entries) > 0 {
			out.Debug().Int("n_entries", len(entries)).Str("remote", c.Name).Msg("sending append entries")
		} else {
			out.Trace().Str("remote", c.Name).Msg("sending heartbeat")
		}

		req := &pb.AppendRequest{
			Term:         term,
			Leader:       leader,
			PrevLogIndex: prev.Index,
			PrevLogTerm:  prev.Term,
			LeaderCommit: commitIndex,
			Entries:      entries,
		}

		// If we're not online, attempt to connect
		if !c.online {
			if err := c.Reset(); err != nil {
				out.Debug().Err(err).Str("remote", c.Name).Msg("not online, attempting to connect")
				return
			}
		}
		if c.stream != nil {
			if err := c.stream.Send(req); err != nil {
				// errors are not fatal, just go offline
				return
			}
		}
	}()

	return nil
}

//===========================================================================
// Connection Handlers
//===========================================================================

// Connect to the remote using the specified timeout. Connect is usually not
// explicitly called, but is instead connected when a message is sent.
func (c *Remote) Connect() (err error) {
	addr := c.Endpoint(true)
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	if c.conn, err = grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials())); err != nil {
		return fmt.Errorf("could not connect to '%s': %s", addr, err.Error())
	}

	// NOTE: do not set online to true until after a response from remote.
	c.client = pb.NewRaftClient(c.conn)

	// Create the append entries stream
	if c.stream, err = c.client.AppendEntries(context.Background()); err != nil {
		return fmt.Errorf("could not create append entries stream to '%s': %s", addr, err.Error())
	}

	// Run the stream event dispatcher
	go c.streamListener()
	return nil
}

// Close the connection to the remote and cleanup the client
func (c *Remote) Close() (err error) {

	// Ensure a valid state after close
	defer func() {
		c.conn = nil
		c.client = nil
		c.stream = nil
		c.online = false
	}()

	if c.stream != nil {
		if err = c.stream.CloseSend(); err != nil {
			return fmt.Errorf("could not close stream to %s: %s", c.Name, err)
		}
	}

	// Don't cause any panics if already closed
	if c.conn == nil {
		return nil
	}

	if err = c.conn.Close(); err != nil {
		return fmt.Errorf("could not close connection to %s: %s", c.Name, err)
	}

	return nil
}

// Reset the connection to the remote
func (c *Remote) Reset() (err error) {
	if err = c.Close(); err != nil {
		return err
	}

	return c.Connect()
}

//===========================================================================
// Message sending management
//===========================================================================

// Send accepts a closure and performs before, RPC call, and after handlers
// for error checking and context management. Used to wrap the GRPC client.
func (c *Remote) send(rpc func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	ctx, cancel, err := c.beforeSend()
	if err != nil {
		return nil, err
	}
	defer cancel()

	rep, err := rpc(ctx)
	if c.afterSend(err) != nil {
		return nil, err
	}

	return rep, nil
}

// Create the context and handle connections
func (c *Remote) beforeSend() (context.Context, context.CancelFunc, error) {
	// If we're not online, attempt to connect
	if !c.online {
		if err := c.Reset(); err != nil {
			out.Debug().Err(err).Str("remote", c.Name).Msg("not online, attempting to connect")
			return nil, nil, err
		}
	}

	// Create the context of the GRPC request
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	return ctx, cancel, nil
}

// Handle errors and connections from non responses
func (c *Remote) afterSend(err error) error {
	if err != nil {
		out.Debug().Err(err).Str("remote", c.Name).Msg("could not send message")

		if c.online {
			// We were online and now we're offline
			out.Warn().Err(err).Str("remote", c.Name).Str("endpoint", c.Endpoint(false)).Msg("grpc connection to remote is offline")
		}
		c.online = false
	} else {
		if !c.online {
			// We were offline and now we're online
			out.Info().Str("remote", c.Name).Str("endpoint", c.Endpoint(false)).Msg("grpc connection to remote is online")
		}
		c.online = true
	}

	return err
}

// Dispatch fatal errors as an error event to the actor, in a go routine so
// that the remote can still be accessed or managed (e.g. closed).
//
//lint:ignore U1000 keeping this function stub for future use.
func (c *Remote) error(err error) {
	go func() {
		c.actor.Dispatch(&event{
			etype:  ErrorEvent,
			source: c,
			value:  err,
		})
	}()
}

func (c *Remote) streamListener() {
	var (
		rep *pb.AppendReply
		err error
	)

	for {
		if c.stream == nil {
			return
		}
		if rep, err = c.stream.Recv(); err != nil {
			if err != io.EOF {
				out.Debug().Err(err).Str("remote", c.Name).Msg("stream closed prematurely")
			} else {
				out.Debug().Err(err).Str("remote", c.Name).Msg("stream closed by remote")
			}

			if c.online {
				// We were online and now we're offline
				out.Warn().Err(err).Str("remote", c.Name).Str("endpoint", c.Endpoint(false)).Msg("grpc connection to remote is offline")
			}
			c.online = false
			return
		}

		if !c.online {
			// We were offline and now we're online
			out.Info().Str("remote", c.Name).Str("endpoint", c.Endpoint(false)).Msg("grpc connection to remote is online")
			c.online = true
		}

		// Dispatch the append reply event
		c.actor.Dispatch(&event{
			etype:  AppendReplyEvent,
			source: c,
			value:  rep,
		})
	}
}
