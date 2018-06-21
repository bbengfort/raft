/*
Package raft implements the Raft consensus algorithm.
*/
package raft

import (
	"log"
	"math/rand"
	"os"
	"time"

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
	rand.Seed(time.Now().Unix())

	// Initialize our debug logging with our prefix
	SetLogger(log.New(os.Stdout, "[raft] ", log.Lmicroseconds))

	// Stop the grpc verbose logging
	grpclog.SetLogger(noplog.New())
}
