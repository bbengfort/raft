package raft

import (
	"context"
	"io"

	"github.com/bbengfort/raft/pb"
)

// Commit a client request to append some entry to the log.
func (r *Replica) Commit(ctx context.Context, in *pb.CommitRequest) (*pb.CommitReply, error) {
	// If the replica is not the leader, forward to the leader.
	if r.leader != r.Name {
		return r.makeRedirect(), nil
	}

	// Create a channel to wait for the commit handler
	source := make(chan *pb.CommitReply, 1)

	// Dispatch the event and wait for it to be handled
	event := &event{etype: CommitRequestEvent, source: source, value: in}
	if err := r.Dispatch(event); err != nil {
		return nil, err
	}

	out := <-source
	return out, nil
}

// helper function to create a redirect message.
func (r *Replica) makeRedirect() *pb.CommitReply {
	var errMsg string
	if r.leader != "" {
		errMsg = "redirect"
	} else {
		errMsg = "no leader available"
	}

	return &pb.CommitReply{
		Success: false, Redirect: r.leader, Error: errMsg, Entry: nil,
	}
}

// RequestVote from peers whose election timeout has elapsed.
func (r *Replica) RequestVote(ctx context.Context, in *pb.VoteRequest) (*pb.VoteReply, error) {
	// Create a channel to wait for event handler on.
	source := make(chan *pb.VoteReply, 1)

	// Dispatch the event received and wait for it to be handled
	event := &event{
		etype:  VoteRequestEvent,
		source: source,
		value:  in,
	}
	if err := r.Dispatch(event); err != nil {
		return nil, err
	}

	out := <-source
	return out, nil
}

// AppendEntries from leader for either heartbeat or consensus.
func (r *Replica) AppendEntries(stream pb.Raft_AppendEntriesServer) (err error) {

	// Keep receiving messages on the stream and sending replies after each
	// message is sent on the stream.
	for {
		var in *pb.AppendRequest
		if in, err = stream.Recv(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Create a channel to wait for event handler on.
		source := make(chan *pb.AppendReply, 1)

		// Dispatch the event received and wait for it to be handled
		event := &event{
			etype:  AppendRequestEvent,
			source: source,
			value:  in,
		}
		if err = r.Dispatch(event); err != nil {
			return err
		}

		// Wait for the event to be handled before receiving the next
		// message on the stream; this ensures that the order of messages
		// received matches the order of replies sent.
		out := <-source
		if err = stream.Send(out); err != nil {
			return err
		}

	}
}
