package raft

import (
	pb "github.com/bbengfort/raft/api/v1beta1"
	"github.com/rs/zerolog/log"
)

func (r *Replica) onHeartbeatTimeout(e Event) error {
	for _, peer := range r.remotes {
		if err := peer.AppendEntries(r.leader, r.term, r.log); err != nil {
			return err
		}
	}
	return nil
}

func (r *Replica) onElectionTimeout(e Event) error {
	r.setState(Candidate)
	return nil
}

func (r *Replica) onCommitRequest(e Event) (err error) {
	var (
		ok    bool
		req   *pb.CommitRequest
		con   chan *pb.CommitReply
		entry *pb.LogEntry
	)

	if con, ok = e.Source().(chan *pb.CommitReply); !ok {
		return ErrEventSourceError
	}

	// If the replica is not the leader, forward to the leader.
	if r.leader != r.Name {
		con <- r.makeRedirect()
		return nil
	}

	// Otherwise append the entry and send out append entries.
	if req, ok = e.Value().(*pb.CommitRequest); !ok {
		return ErrEventTypeError
	}

	if entry, err = r.log.Create(req.Name, req.Value, r.term); err != nil {
		return err
	}

	// Interrupt the heartbeat and send append entries
	r.ticker.Interrupt(HeartbeatTimeout)

	for _, peer := range r.remotes {
		if err = peer.AppendEntries(r.leader, r.term, r.log); err != nil {
			return err
		}
	}

	// Tie the entry and the source together so reply is sent on commit/drop.
	r.clients[entry.Index] = con
	return nil
}

func (r *Replica) onAggregatedCommitRequest(ae Event) (err error) {
	var (
		ok    bool
		reqs  []Event
		req   *pb.CommitRequest
		con   chan *pb.CommitReply
		entry *pb.LogEntry
	)

	// Get all requests from the event
	if reqs, ok = ae.Value().([]Event); !ok {
		return ErrEventTypeError
	}

	// Tell the world about the aggregation
	log.Info().Int("n_requests", len(reqs)).Msg("aggregating commit requests into a single append entry")

	// Handle each request by redirecting to the leader or creating an entry
	// in the log and associating the client with the entries index for reply.
	for _, e := range reqs {
		// Get the commit reply connection
		if con, ok = e.Source().(chan *pb.CommitReply); !ok {
			return ErrEventSourceError
		}

		// If the replica is not the leader, forward the client
		if r.leader != r.Name {
			con <- r.makeRedirect()
			continue
		}

		// Otherwise create an entry in the log
		if req, ok = e.Value().(*pb.CommitRequest); !ok {
			return ErrEventTypeError
		}

		if entry, err = r.log.Create(req.Name, req.Value, r.term); err != nil {
			return err
		}

		// Tie the entry and the source together so reply is sent on commit/drop.
		r.clients[entry.Index] = con
	}

	// If we're not leader, we're done sending redirects, so exit
	if r.leader != r.Name {
		return nil
	}

	// Interrupt the heartbeat and send append entries with all new entries
	r.ticker.Interrupt(HeartbeatTimeout)

	for _, peer := range r.remotes {
		if err = peer.AppendEntries(r.leader, r.term, r.log); err != nil {
			return err
		}
	}

	// Track the number of aggregations
	r.Metrics.Aggregation(len(reqs))

	return nil
}

func (r *Replica) onVoteRequest(e Event) (err error) {
	var (
		ok  bool
		req *pb.VoteRequest
		con chan *pb.VoteReply
	)

	if req, ok = e.Value().(*pb.VoteRequest); !ok {
		return ErrEventTypeError
	}
	if con, ok = e.Source().(chan *pb.VoteReply); !ok {
		return ErrEventSourceError
	}

	log.Debug().
		Str("name", r.Name).
		Str("candidate", req.Candidate).
		Uint64("term", req.Term).
		Msg("received vote request")

	// If RPC term is greater than local term, update and set state to follower.
	if _, err := r.CheckRPCTerm(req.Term); err != nil {
		return err
	}

	// Construct the reply
	rep := &pb.VoteReply{Remote: r.Name, Term: r.term, Granted: false}

	// Determine whether or not to grant the vote
	if req.Term >= r.term {
		if r.votedFor == "" || r.votedFor == req.Candidate {
			// Ensure the log is as up to date as the candidate's.
			if r.log.AsUpToDate(req.LastLogIndex, req.LastLogTerm) {
				log.Info().Str("candidate", req.Candidate).Uint64("term", req.Term).Msg("voting for candidate")
				r.ticker.Interrupt(ElectionTimeout)
				rep.Granted = true
				r.votedFor = req.Candidate
			} else {
				log.Debug().Str("candidate", req.Candidate).Msg("log is not as up to date as candidate")
			}
		} else {
			log.Debug().Str("voted_for", r.votedFor).Uint64("term", r.term).Msg("already voted in current term")
		}
	}

	// Send the reply
	con <- rep
	return nil
}

func (r *Replica) onVoteReply(e Event) error {
	rep, ok := e.Value().(*pb.VoteReply)
	if !ok {
		return ErrEventTypeError
	}

	// If RPC term is greater than local term, update and set state to follower.
	if _, err := r.CheckRPCTerm(rep.Term); err != nil {
		return err
	}

	// If we're still a candidate, update vote and determine election
	if r.state == Candidate {
		log.Debug().
			Str("candidate", r.Name).
			Str("from", rep.Remote).
			Bool("granted", rep.Granted).
			Uint64("term", rep.Term).
			Msg("received vote")

		r.votes.Vote(rep.Remote, rep.Granted)
		if r.votes.Passed() {
			return r.setState(Leader)
		}
	}

	return nil
}

func (r *Replica) onAppendRequest(e Event) error {
	var (
		ok  bool
		req *pb.AppendRequest
		con chan *pb.AppendReply
	)

	if req, ok = e.Value().(*pb.AppendRequest); !ok {
		return ErrEventTypeError
	}
	if con, ok = e.Source().(chan *pb.AppendReply); !ok {
		return ErrEventSourceError
	}

	// If RPC term is greater than local term, update and set state to follower.
	if _, err := r.CheckRPCTerm(req.Term); err != nil {
		return err
	}

	if len(req.Entries) == 0 {
		log.Trace().Str("leader", req.Leader).Uint64("term", req.Term).Msg("heartbeat received")

	} else {
		log.Debug().
			Str("leader", req.Leader).
			Uint64("term", req.Term).
			Uint64("prev_log_index", req.PrevLogIndex).
			Int("entries", len(req.Entries)).
			Msg("appending entries after previous log index in term")
	}

	// Construct the reply
	rep := &pb.AppendReply{
		Remote: r.Name, Term: r.term, Success: false,
		Index: r.log.LastApplied(), CommitIndex: r.log.CommitIndex(),
	}

	// Ensure reply is sent when function is concluded
	defer func() { con <- rep }()

	// Check to make sure that the append entires term is not stale
	if req.Term < r.term {
		log.Debug().Uint64("remote_term", req.Term).Uint64("local_term", r.term).Msg("append entries term is stale")
		return nil
	}

	// Interrupt the election timeout and set sender as leader
	r.ticker.Interrupt(ElectionTimeout)
	r.leader = req.Leader

	// Determine if we should append entries
	if err := r.log.Truncate(req.PrevLogIndex, req.PrevLogTerm); err != nil {
		log.Debug().Err(err).Msg("could not truncate log")
		return nil
	}

	// Perform the append entries
	if len(req.Entries) > 0 {
		if err := r.log.Append(req.Entries...); err != nil {
			return err
		}
	}

	// If leader commit > local commit, update our commit index
	if req.LeaderCommit > r.log.CommitIndex() {
		var commitIndex uint64
		if req.LeaderCommit > r.log.lastApplied {
			commitIndex = r.log.lastApplied
		} else {
			commitIndex = req.LeaderCommit
		}

		if err := r.log.Commit(commitIndex); err != nil {
			return err
		}
	}

	// At this point, append entries is accepted
	rep.Success = true
	rep.Index = r.log.LastApplied()
	rep.CommitIndex = r.log.CommitIndex()
	return nil
}

func (r *Replica) onAppendReply(e Event) error {
	rep, ok := e.Value().(*pb.AppendReply)
	if !ok {
		return ErrEventTypeError
	}

	// If RPC term is greater than local term, update and set state to follower.
	if _, err := r.CheckRPCTerm(rep.Term); err != nil {
		return err
	}

	// If we're no longer the leader, stop processing reply
	if r.state != Leader {
		return nil
	}

	// Update remote state based on success or failure
	// TODO: review for correctness
	peer := r.remotes[rep.Remote]
	if rep.Success {
		peer.nextIndex = rep.Index + 1
		peer.matchIndex = rep.Index
	} else {
		if rep.Index < peer.nextIndex {
			peer.nextIndex = rep.Index + 1
		} else {
			peer.nextIndex = rep.Index
		}
		return nil
	}

	// Decide if we can commit
	return r.CheckCommits()
}
