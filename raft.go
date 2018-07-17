/*
Package raft implements the Raft consensus algorithm.
*/
package raft

import (
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/bbengfort/raft/pb"
	"github.com/bbengfort/x/noplog"
	"google.golang.org/grpc/grpclog"
)

//===========================================================================
// Package Initialization
//===========================================================================

// PackageVersion of the current Raft implementation
const PackageVersion = "0.1"

// Initialize the package and random numbers, etc.
func init() {
	// Set the random seed to something different each time.
	rand.Seed(time.Now().UnixNano())

	// Initialize our debug logging with our prefix
	SetLogger(log.New(os.Stdout, "[raft] ", log.Lmicroseconds))
	cautionCounter = new(counter)
	cautionCounter.init()

	// Stop the grpc verbose logging
	grpclog.SetLogger(noplog.New())
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

	// Set the logging level and the random seed
	SetLogLevel(uint8(config.LogLevel))
	if config.Seed != 0 {
		debug("setting random seed to %d", config.Seed)
		rand.Seed(config.Seed)
	}

	// Create and initialize the replica
	replica = new(Replica)
	replica.config = config
	replica.remotes = make(map[string]*Remote)
	replica.clients = make(map[uint64]chan *pb.CommitReply)
	replica.log = NewLog(replica)

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
	info("raft replica with %d remote peers created", len(replica.remotes))
	return replica, nil
}
