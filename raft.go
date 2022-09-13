/*
Package raft implements the Raft consensus algorithm.
*/
package raft

import (
	"math/rand"
	"os"
	"time"

	pb "github.com/bbengfort/raft/api/v1beta1"
	"github.com/bbengfort/x/noplog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/grpclog"
)

//===========================================================================
// Package Initialization
//===========================================================================

// Initialize the package and random numbers, etc.
func init() {
	// Set the random seed to something different each time.
	rand.Seed(time.Now().UnixNano())

	// Stop the grpc verbose logging
	//lint:ignore SA1019 noplog doesn't implement the V2 interface
	grpclog.SetLogger(noplog.New())

	// Initialize zerolog for server-side process logging
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

// StateMachine implements a handler for applying commands when they are
// committed or for dropping commands if they are truncated from the log.
type StateMachine interface {
	CommitEntry(entry *pb.LogEntry) error
	DropEntry(entry *pb.LogEntry) error
}

//===========================================================================
// New Raft Instance
//===========================================================================

// New Raft replica with the specified config.
func New(options *Config) (replica *Replica, err error) {
	// Create a new configuration from defaults, configuration file, and
	// the environment; then verify it, returning any errors.
	config := new(Config)
	if err = config.Load(); err != nil {
		return nil, err
	}

	// Update the configuration with the passed in configuration.
	if err = config.Update(options); err != nil {
		return nil, err
	}

	// Configure logging (will modify logging globally for all packages!)
	zerolog.SetGlobalLevel(config.GetLogLevel())
	if config.ConsoleLog {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Set the random seed
	if config.Seed != 0 {
		log.Debug().Int64("seed", config.Seed).Msg("setting random seed")
		rand.Seed(config.Seed)
	}

	// Create and initialize the replica
	replica = new(Replica)
	replica.config = config
	replica.remotes = make(map[string]*Remote)
	replica.clients = make(map[uint64]chan *pb.CommitReply)
	replica.log = NewLog(replica)
	replica.Metrics = NewMetrics()

	// Create the local replica definition
	replica.Peer, err = config.GetPeer()
	if err != nil {
		return nil, err
	}

	// Create the remotes from peers
	for _, peer := range config.Peers {
		// Do not store local host in remotes
		if replica.Name == peer.Name {
			continue
		}

		// Create the remote connection and client
		replica.remotes[peer.Name], err = NewRemote(peer, replica)
		if err != nil {
			return nil, err
		}
	}

	// Create the ticker from the configuration
	tick, err := config.GetTick()
	if err != nil {
		return nil, err
	}
	replica.ticker = NewTicker(replica, tick)

	// Set state to initialized
	replica.setState(Initialized)
	log.Info().Str("name", replica.Name).Int("nPeers", len(replica.remotes)).Msg("raft replica created")
	return replica, nil
}
