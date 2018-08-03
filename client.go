package raft

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/bbengfort/raft/pb"
	"github.com/bbengfort/x/peers"
	"google.golang.org/grpc"
)

// DefaultRetries specifies the number of times to attempt a commit.
const DefaultRetries = 3

// CommitClient is an interface for both unary and streaming rpc client types.
type CommitClient interface {
	Commit(name string, value []byte) (entry *pb.LogEntry, err error)
}

// NewClient creates a new raft client to conect to a quorum.
func NewClient(options *Config, streaming bool) (CommitClient, error) {
	// Create a new configuration from defaults, configuration file, and the
	// environment; verify it returning any errors.
	config := new(Config)
	if err := config.Load(); err != nil {
		return nil, err
	}

	// Update the configuration with the passed in options
	if err := config.Update(options); err != nil {
		return nil, err
	}

	// Compute the identity
	var identity string
	hostname, _ := config.GetName()
	if hostname != "" {
		identity = fmt.Sprintf("%s-%04X", hostname, rand.Intn(0x10000))
	} else {
		identity = fmt.Sprintf("%04X-%04X", rand.Intn(0x10000), rand.Intn(0x10000))
	}

	var client CommitClient
	if streaming {
		client = &Client{config: config, identity: identity}
	} else {
		client = &StreamingClient{Client: Client{config: config, identity: identity}}
	}

	return client, nil
}

// Client maintains network information embedded in the configuration to
// connect to a Raft consensus quorum and make commit requests.
type Client struct {
	config   *Config
	conn     *grpc.ClientConn
	client   pb.RaftClient
	identity string
}

//===========================================================================
// Request API
//===========================================================================

// Commit a name and value to the distributed log.
func (c *Client) Commit(name string, value []byte) (entry *pb.LogEntry, err error) {
	// Create the request
	req := &pb.CommitRequest{Identity: c.identity, Name: name, Value: value}

	// Send the request
	rep, err := c.send(req, DefaultRetries)
	if err != nil {
		return nil, err
	}

	return rep.Entry, nil
}

// Send the commit request, handling redirects for the maximum number of tries.
func (c *Client) send(req *pb.CommitRequest, retries int) (*pb.CommitReply, error) {
	// Don't attempt if there are no more retries.
	if retries <= 0 {
		return nil, ErrRetries
	}

	// Connect if not connected
	if !c.isConnected() {
		if err := c.connect(""); err != nil {
			return nil, err
		}
	}

	// Create the context
	timeout, err := c.config.GetTimeout()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rep, err := c.client.Commit(ctx, req)
	if err != nil {
		if retries > 1 {
			// If there is an error connecting to the current host, try a
			// different host on the network.
			if err = c.connect(""); err != nil {
				return nil, err
			}
			return c.send(req, retries-1)
		}
		return nil, err
	}

	if !rep.Success {
		if rep.Redirect != "" {
			// Redirect to the specified leader.
			if err := c.connect(rep.Redirect); err != nil {
				return nil, err
			}
			return c.send(req, retries-1)
		}
		return nil, errors.New(rep.Error)
	}

	return rep, nil
}

//===========================================================================
// Connection Handlers
//===========================================================================

// Close the connection to the remote host
func (c *Client) close() error {
	// Ensure a valid state after close
	defer func() {
		c.conn = nil
		c.client = nil
	}()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// Connect to the remote using the specified timeout. If a remote is not
// specified (e.g. empty string) then a random replica is selected from the
// configuration to connect to.
func (c *Client) connect(remote string) (err error) {
	// Close connection if one is already open.
	c.close()

	// Get the peer by name or select a random peer.
	var host *peers.Peer
	if host, err = c.selectRemote(remote); err != nil {
		return err
	}

	// Parse timeout from configuration for the connection
	var timeout time.Duration
	if timeout, err = c.config.GetTimeout(); err != nil {
		return err
	}

	// Connect to the remote's address
	addr := host.Endpoint(false)
	if c.conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(timeout)); err != nil {
		return fmt.Errorf("could not connect to '%s': %s", addr, err.Error())
	}

	// Create the gRPC client
	c.client = pb.NewRaftClient(c.conn)
	return nil
}

// Returns true if a client and a connection exist
func (c *Client) isConnected() bool {
	return c.client != nil && c.conn != nil
}

// Returns a random remote from the configuration if the remote is not
// specified, otherwise searches for the remote by name.
func (c *Client) selectRemote(remote string) (*peers.Peer, error) {

	if remote == "" {
		if len(c.config.Peers) == 0 {
			return nil, ErrNoNetwork
		}

		idx := rand.Intn(len(c.config.Peers))
		return &c.config.Peers[idx], nil
	}

	for _, peer := range c.config.Peers {
		if peer.Name == remote {
			return &peer, nil
		}
	}

	return nil, fmt.Errorf("could not find remote '%s' in configuration", remote)
}

//===========================================================================
// Streaming Client
//===========================================================================

// StreamingClient implements the ClientStream RPC to the Raft service.
type StreamingClient struct {
	Client
	stream pb.Raft_CommitStreamClient
}

// Commit a name and value to the distributed log.
func (c *StreamingClient) Commit(name string, value []byte) (entry *pb.LogEntry, err error) {
	// Create the request
	req := &pb.CommitRequest{Identity: c.identity, Name: name, Value: value}

	// Send the request
	rep, err := c.send(req, DefaultRetries)
	if err != nil {
		return nil, err
	}

	return rep.Entry, nil
}

// Send the commit request on the stream, handling redirects and outages for
// the maximum number of tries. Implements ping-pong streming, sends and waits
// for a response from the server.
func (c *StreamingClient) send(req *pb.CommitRequest, retries int) (*pb.CommitReply, error) {
	// Don't attempt if there are no more retries.
	if retries <= 0 {
		return nil, ErrRetries
	}

	// Connect if not connected
	if !c.isConnected() {
		if err := c.connect(""); err != nil {
			return nil, err
		}
	}

	// Send the message on the stream
	if err := c.stream.Send(req); err != nil {
		// try again with a different host until retries are exhausted
		c.close()
		return c.send(req, retries-1)
	}

	// Listen for a reply on the stream
	rep, err := c.stream.Recv()
	if err != nil {
		// try again with a different host until retries are exhausted
		c.close()
		return c.send(req, retries-1)
	}

	if !rep.Success {
		if rep.Redirect != "" {
			// Redirect to the specified leader.
			if err := c.connect(rep.Redirect); err != nil {
				return nil, err
			}
			return c.send(req, retries-1)
		}
		return nil, errors.New(rep.Error)
	}

	return rep, nil
}

// Connect the client then the stream
func (c *StreamingClient) connect(remote string) (err error) {
	if err = c.Client.connect(remote); err != nil {
		return err
	}

	c.stream, err = c.client.CommitStream(context.Background())
	return err
}

// Close the stream and the client connection
func (c *StreamingClient) close() (err error) {
	defer func() {
		c.stream = nil
	}()

	if c.stream != nil {
		if err = c.stream.CloseSend(); err != nil {
			return err
		}
	}

	return c.Client.close()
}

// Returns true if a client, connection, and a stream exist
func (c *StreamingClient) isConnected() bool {
	return c.client != nil && c.conn != nil && c.stream != nil
}
