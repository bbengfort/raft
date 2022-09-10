package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// NewBenchmark creates either a blast or a simple benchmark depending on the
// blast boolean flag. In blast mode, N operations is executed simultaneously
// against the cluster putting a unique key with a random value of size S.
// In simple mode, C workers executes N requests each, putting a unique key
// with a random value of size S. Note that C is ignored in blast mode.
func NewBenchmark(options *Config, addr string, blast bool, N, S, C uint) (bench Benchmark, err error) {
	if blast {
		bench = &BlastBenchmark{
			opts:       options,
			operations: N, dataSize: S,
			benchmark: benchmark{method: "blast"},
		}
	} else {
		bench = &SimpleBenchmark{
			benchmark:  benchmark{method: "simple"},
			opts:       options,
			operations: N, dataSize: S, concurrency: C,
		}
	}

	if err := bench.Run(addr); err != nil {
		return nil, err
	}
	return bench, nil
}

// Benchmark defines the interface for all benchmark runners, both for
// execution as well as the delivery of results. A single benchmark is
// executed once and stores its internal results to be saved to disk.
type Benchmark interface {
	Run(addr string) error           // execute the benchmark, may return an error if already run
	CSV(header bool) (string, error) // returns a CSV representation of the results
	JSON(indent int) ([]byte, error) // returns a JSON representation of the results
}

//===========================================================================
// benchmark
//===========================================================================

// This embedded struct implements shared functionality between many of the
// implemented benchmarks, keeping track of the throughput and the numbber of
// successful or unsuccessful requests.
type benchmark struct {
	method    string          // the name of the benchmark type
	requests  uint64          // the number of successful requests
	failures  uint64          // the number of failed requests
	started   time.Time       // the time the benchmark was started
	duration  time.Duration   // the duration of the benchmark period
	latencies []time.Duration // observed latencies in the number of requests
}

// Complete returns true if requests and duration is greater than 0.
func (b *benchmark) Complete() bool {
	return b.requests > 0 && b.duration > 0
}

// Throughput computes the number of requests (excluding failures) by the
// total duration of the experiment, e.g. the operations per second.
func (b *benchmark) Throughput() float64 {
	if b.duration == 0 {
		return 0.0
	}

	return float64(b.requests) / b.duration.Seconds()
}

// CSV returns a results row delimited by commas as:
//
//	requests,failures,duration,throughput,version,benchmark
//
// If header is specified then string contains two rows with the header first.
func (b *benchmark) CSV(header bool) (string, error) {
	if !b.Complete() {
		return "", errors.New("benchmark has not been run yet")
	}

	row := fmt.Sprintf(
		"%d,%d,%s,%0.4f,%s,%s",
		b.requests, b.failures, b.duration, b.Throughput(), Version(), b.method,
	)

	if header {
		return fmt.Sprintf("requests,failures,duration,throughput,version,benchmark\n%s", row), nil
	}

	return row, nil
}

// JSON returns a results row as a json object, formatted with or without the
// number of spaces specified by indent. Use no indent for JSON lines format.
func (b *benchmark) JSON(indent int) ([]byte, error) {
	data := b.serialize()

	if indent > 0 {
		indent := strings.Repeat(" ", indent)
		return json.MarshalIndent(data, "", indent)
	}

	return json.Marshal(data)
}

// serialize converts the benchmark into a map[string]interface{} -- useful
// for dumping the benchmark as JSON and used from structs that embed benchmark
// to include more data in the results.
func (b *benchmark) serialize() map[string]interface{} {
	data := make(map[string]interface{})

	data["requests"] = b.requests
	data["failures"] = b.failures
	data["duration"] = b.duration.String()
	data["throughput"] = b.Throughput()
	data["version"] = Version()
	data["benchmark"] = b.method

	return data
}

//===========================================================================
// Blast
//===========================================================================

// BlastBenchmark implements Benchmark by sending n Put requests to the specified server
// each in its own thread. It then records the total time it takes to complete
// all n requests and uses that to compute the throughput. Additionally, each
// thread records the latency of each request, so that outlier requests can
// be removed from the blast computation.
//
// Note: this benchmark Puts a unique key and short value to the server, its
// intent is to compute pedal to the metal write throughput.
type BlastBenchmark struct {
	benchmark
	opts       *Config
	operations uint
	dataSize   uint
}

// Run the blast benchmark against the system by putting a unique key and
// small value to the server as fast as possible and measuring the duration.
func (b *BlastBenchmark) Run(addr string) (err error) {
	// N is the number of operations being run
	N := b.operations

	// S is the size of the value to put to the server
	S := b.dataSize

	// Initialize the blast latencies and results (resetting if rerun)
	b.requests = 0
	b.failures = 0
	b.latencies = make([]time.Duration, N)
	results := make([]bool, N)

	// Initialize the keys and values so that it's not part of throughput.
	keys := make([]string, N)
	vals := make([][]byte, N)

	for i := uint(0); i < N; i++ {
		keys[i] = fmt.Sprintf("%X", i)
		vals[i] = make([]byte, S)
		rand.Read(vals[i])
	}

	// Create the wait group for all threads
	group := new(sync.WaitGroup)
	group.Add(int(N))

	// Initialize a single client for all operations and connect.
	var clients *Client
	if clients, err = NewClient(addr, b.opts); err != nil {
		return fmt.Errorf("could not create client: %s", err)
	}

	// Execute the blast operation against the server
	b.started = time.Now()
	for i := uint(0); i < N; i++ {
		go func(k uint) {
			// Make Put request and if there is no error, store true!
			start := time.Now()
			if _, err := clients.Commit(keys[k], vals[k]); err == nil {
				results[k] = true
			}

			// Record the latency of the result, success or failure
			b.latencies[k] = time.Since(start)
			group.Done()
		}(i)
	}

	group.Wait()
	b.duration = time.Since(b.started)

	// Compute successes and failures
	for _, r := range results {
		if r {
			b.requests++
		} else {
			b.failures++
		}
	}

	return nil
}

//===========================================================================
// Simple
//===========================================================================

// SimpleBenchmark implements benchmark by having concurrent workers continuously
// sending requests at the server for a fixed number of requests.
type SimpleBenchmark struct {
	benchmark
	opts        *Config
	operations  uint
	dataSize    uint
	concurrency uint
}

// Run the simple benchmark against the system such that each client puts a
// unique key and small value to the server as quickly as possible.
func (b *SimpleBenchmark) Run(addr string) (err error) {
	n := b.operations  // number of operations per client
	C := b.concurrency // total number of clients
	N := n * C         // total number of operations
	S := b.dataSize    // size of the value to put to the server

	// Initialize benchmark latencies and results (resetting if necessary)
	b.requests, b.failures = 0, 0
	b.latencies = make([]time.Duration, N)
	results := make([]bool, N)

	// Create the wait group for all threads
	group := new(sync.WaitGroup)
	group.Add(int(C))

	// Initialize the clients per routine and connect to server. Note this
	// should be done as late as possible to ensure connections are open for
	// as little time as possible.
	clients := make([]*Client, C)
	for i := uint(0); i < C; i++ {
		if clients[i], err = NewClient(addr, b.opts); err != nil {
			return fmt.Errorf("could not create client %d: %s", i, err)
		}
	}

	// Execute the concurrent workers against the system
	b.started = time.Now()
	for i := uint(0); i < C; i++ {
		go func(k uint) {
			// Execute n requests against the server
			for j := uint(0); j < n; j++ {
				// Compute storage index
				idx := k*C + j

				// Compute unique key and value based on client and request
				key := fmt.Sprintf("%04X-%04X", k, j)
				val := make([]byte, S)
				rand.Read(val)

				// Make Put request, and if no error, store true.
				start := time.Now()
				if _, err := clients[k].Commit(key, val); err == nil {
					results[idx] = true
				}

				// Record the latency of the result
				b.latencies[idx] = time.Since(start)
			}

			// Signal the main thread that we're done
			group.Done()
		}(i)
	}

	// Wait until benchmark is complete
	group.Wait()
	b.duration = time.Since(b.started)

	// Compute successes and failures
	for _, r := range results {
		if r {
			b.requests++
		} else {
			b.failures++
		}
	}

	return nil
}

// CSV returns a results row delimited by commas as:
//
//	concurrency,requests,failures,duration,throughput,version,benchmark
func (b *SimpleBenchmark) CSV(header bool) (csv string, err error) {
	if csv, err = b.benchmark.CSV(header); err != nil {
		return "", err
	}

	if header {
		parts := strings.Split(csv, "\n")
		if len(parts) != 2 {
			return "", errors.New("could not parse benchmark header")
		}
		return fmt.Sprintf("concurrency,%s\n%d,%s", parts[0], b.concurrency, parts[1]), nil
	}

	return fmt.Sprintf("%d,%s", b.concurrency, csv), nil
}

// JSON returns a results row as a json object, formatted with or without the
// number of spaces specified by indent. Use no indent for JSON lines format.
func (b *SimpleBenchmark) JSON(indent int) ([]byte, error) {
	data := b.benchmark.serialize()
	data["concurrency"] = b.concurrency

	if indent > 0 {
		indent := strings.Repeat(" ", indent)
		return json.MarshalIndent(data, "", indent)
	}

	return json.Marshal(data)
}
