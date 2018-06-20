package raft

import "time"

// NewTicker creates a ticker with intervals for each of the timing events in
// the system, computed from a base "tick" rate, defined as the delay between
// heartbeats.
func NewTicker(actor Actor, tick time.Duration) *Ticker {
	ticker := &Ticker{
		timeouts: make(map[EventType]Interval),
	}

	ticker.timeouts[HeartbeatTimeout] = NewFixedInterval(actor, tick, HeartbeatTimeout)
	ticker.timeouts[ElectionTimeout] = NewRandomInterval(actor, 2*tick, 4*tick, ElectionTimeout)

	return ticker
}

// Ticker implements intervals for all timing events based on the tick
// parameter set by the user. External objects can register for timing events
// in order to handle ticks, as well as to manage individual tickers.
type Ticker struct {
	timeouts map[EventType]Interval
}

// Start the specified ticker
func (t *Ticker) Start(etype EventType) bool {
	return t.timeouts[etype].Start()
}

// Stop the specified ticker
func (t *Ticker) Stop(etype EventType) bool {
	return t.timeouts[etype].Stop()
}

// StopAll of the currently running tickers.
func (t *Ticker) StopAll() int {
	stopped := 0
	for _, ticker := range t.timeouts {
		if ticker.Stop() {
			stopped++
		}
	}
	return stopped
}

// Interrupt the specified ticker
func (t *Ticker) Interrupt(etype EventType) bool {
	return t.timeouts[etype].Interrupt()
}

// Running determines if the specified ticker is running.
func (t *Ticker) Running(etype EventType) bool {
	return t.timeouts[etype].Running()
}
