package raft

import (
	"fmt"
	"sync"
	"time"

	"github.com/bbengfort/x/stats"
)

// Metrics tracks the measurable statistics of the system over time from the
// perspective of the local replica -- e.g. how many accesses over a specific
// time period.
type Metrics struct {
	sync.RWMutex
	started     time.Time         // The time of the first client message
	finished    time.Time         // The time of the last client message
	requests    uint64            // Number of requests made to server
	commits     uint64            // The number of committed entries
	drops       uint64            // The number of dropped entries
	clients     map[string]bool   // The unique clients seen
	aggregation *stats.Statistics // Track the number and size of aggregations
}

// NewMetrics creates the metrics data store
func NewMetrics() *Metrics {
	return &Metrics{
		clients:     make(map[string]bool),
		aggregation: new(stats.Statistics),
	}
}

// Request registers a new client request
func (m *Metrics) Request(client string) {
	m.Lock()
	defer m.Unlock()

	m.clients[client] = true
	m.requests++

	if m.started.IsZero() {
		m.started = time.Now()
	}
}

// Complete is called when the request is responded to
func (m *Metrics) Complete(commit bool) {
	m.Lock()
	defer m.Unlock()
	if commit {
		m.commits++
	} else {
		m.drops++
	}
	m.finished = time.Now()
}

// Aggregation is called when an aggregation occurs. No need for synchronization
// here since the stats object is synchronized.
func (m *Metrics) Aggregation(n int) {
	m.aggregation.Update(float64(n))
}

// Dump the metrics to JSON
func (m *Metrics) Dump(path string, extra map[string]interface{}) (err error) {
	m.RLock()
	defer m.RUnlock()

	data := make(map[string]interface{})

	// Append extra information
	for key, val := range extra {
		data[key] = val
	}

	data["metric"] = "server"
	data["version"] = PackageVersion
	data["started"] = m.started.Format(time.RFC3339Nano)
	data["finished"] = m.finished.Format(time.RFC3339Nano)
	data["commits"] = m.commits
	data["drops"] = m.drops
	data["clients"] = len(m.clients)
	data["throughput"] = m.throughput()
	data["duration"] = m.duration().String()
	data["aggregation"] = m.aggregation.Serialize()

	return appendJSON(path, data)
}

// String returns a summary of the access metrics
func (m *Metrics) String() string {
	m.RLock()
	defer m.RUnlock()

	return fmt.Sprintf(
		"%d commits, %d drops in %s -- %0.3f commits/sec",
		m.commits, m.drops, m.duration(), m.throughput(),
	)
}

// Duration computes the amount of time accesses were received.
func (m *Metrics) duration() time.Duration {
	return m.finished.Sub(m.started)
}

// Throughput computes the number of commits per second.
func (m *Metrics) throughput() float64 {
	duration := m.duration()
	if duration == 0 || m.commits == 0 {
		return 0.0
	}

	return float64(m.commits) / duration.Seconds()
}
