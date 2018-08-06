package raft

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
)

// NewBenchmark creates a benchmark with the specified number of clients and
// requests per client, then executes the benchjmark against the quorum.
func NewBenchmark(options *Config, nclients int, requestsPerClient uint64, blast bool) (*Benchmark, error) {
	benchmark := &Benchmark{
		config: options, nClients: nclients, requests: requestsPerClient, blast: blast,
	}
	if err := benchmark.Run(); err != nil {
		return nil, err
	}
	return benchmark, nil
}

// Benchmark runs multiple concurrent clients making commit requests against
// the Raft quorum, by issuing a fixed number of requests per client. The
// throughput is computed as the number of operations per second the benchmark
// achieves while it is running.
type Benchmark struct {
	config   *Config       // The Raft quorum configuration
	nClients int           // The number of concurrent clients
	requests uint64        // The total number of successful commits made
	started  time.Time     // The time the benchmark was started
	duration time.Duration // The duration of the benchmark period
	blast    bool          // If true, all requests are sent in their own thread
}

// Run the benchmark in either fixed duration or maximum commits mode; using
// the clients to execute requests against the quorum. An error is returned
// if both duration and commits are set to 0 (e.g. no benchmark mode is
// specified) or if the benchmark has already been executed.
func (b *Benchmark) Run() error {
	group := new(errgroup.Group)

	b.started = time.Now()

	if b.blast {
		// NOTE: nClients is NOT ignored in blast mode
		requests := b.NumRequests()
		for i := uint64(0); i < requests; i++ {
			group.Go(b.runBlast)
		}
	} else {
		for i := 0; i < b.nClients; i++ {
			group.Go(b.runClient)
		}
	}

	group.Wait()
	b.duration = time.Since(b.started)
	return nil
}

// Throughput returns the number of operations per second.
func (b *Benchmark) Throughput() float64 {
	if b.duration == 0 {
		return 0.0
	}

	return float64(b.NumRequests()) / b.duration.Seconds()
}

// NumClients returns the number of concurrent clients.
func (b *Benchmark) NumClients() uint64 {
	return uint64(b.nClients)
}

// NumRequests returns the total number of commit requests sent.
func (b *Benchmark) NumRequests() uint64 {
	return b.NumClients() * uint64(b.requests)
}

// Duration returns the amount of time it took to send all messages.
func (b *Benchmark) Duration() time.Duration {
	return b.duration
}

// String returns a csv string of nclients,commits,duration,throughput,version
func (b *Benchmark) String() string {
	return fmt.Sprintf(
		"%d,%d,%s,%0.4f,%s",
		b.NumClients(),
		b.NumRequests(),
		b.Duration(),
		b.Throughput(),
		PackageVersion,
	)
}

// Dump the benchmark in JSONlines format to disk.
func (b *Benchmark) Dump(path string) error {
	data := make(map[string]interface{})

	data["metric"] = "client"
	data["started"] = b.started.Format(time.RFC3339Nano)
	data["duration"] = b.Duration().String()
	data["n_clients"] = b.NumClients()
	data["n_requests"] = b.NumRequests()
	data["throughput"] = b.Throughput()
	data["version"] = PackageVersion
	data["hostname"], _ = os.Hostname()

	return appendJSON(path, data)
}

func (b *Benchmark) runClient() (err error) {

	var (
		client *Client
		key    string
		val    []byte
	)

	// Create the client to execute the requests
	if client, err = NewClient(b.config); err != nil {
		return err
	}

	// Send commit requests to the server
	for j := uint64(0); j < b.requests; j++ {
		key = fmt.Sprintf("%s-%04X", client.identity, j)
		val = []byte(time.Now().String())
		if _, err = client.Commit(key, val); err != nil {
			return err
		}
	}
	return nil
}

func (b *Benchmark) runBlast() (err error) {
	// Create the client to execute the requests
	var client *Client
	if client, err = NewClient(b.config); err != nil {
		return err
	}

	// Send the commit request
	if _, err = client.Commit(client.identity, []byte(time.Now().String())); err != nil {
		return err
	}
	return nil
}
