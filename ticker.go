package raft

// Ticker implements intervals for all timing events based on the tick
// parameter set by the user. External objects can register for timing events
// in order to handle ticks, as well as to manage individual tickers.
type Ticker struct{}
