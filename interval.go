package raft

import (
	"math/rand"
	"time"
)

//===========================================================================
// Interval Interface
//===========================================================================

// Interval is an interface that specifies the behavior of time based event
// dispatchers. A single interval object dispatches a single event type, to
// which callbacks from any go routine can be registered. The event is
// dispatched on schedule - the interval can either be fixed or stochastic.
// Fixed intervals resechedule themselves for a fixed delay after all
// callbacks are called. Stochastic intervals select a random delay in a
// configured range to schedule the next event after all callbacks.
//
// Interval objects can be started and stopped. On start, the interval
// schedules the next event after the delay returned by GetDelay(). On stop
// no events will be dispatched by the handler. Intervals can be
// interrupted which resets the timer to a new delay. Timer state (running or
// not running) can be determined by the Running() method.
type Interval interface {
	Start() bool                // start the interval to periodically call its function
	Stop() bool                 // stop the interval, the function will not be called
	Interrupt() bool            // interrupt the interval, setting it to the next period
	Running() bool              // whether or not the interval is running
	GetDelay() time.Duration    // the duration of the current interval period
	Register(callback Callback) // register a handler for the interval event
}

// NewFixedInterval creates and initializes a new fixed interval.
func NewFixedInterval(delay time.Duration, etype EventType) *FixedInterval {
	return &FixedInterval{
		delay:       delay,
		etype:       etype,
		initialized: true,
		timer:       nil,
	}
}

// NewRandomInterval creates and initializes a new random interval.
func NewRandomInterval(minDelay, maxDelay time.Duration, etype EventType) *RandomInterval {
	return &RandomInterval{
		minDelay: int64(minDelay),
		maxDelay: int64(maxDelay),
		FixedInterval: FixedInterval{
			etype:       etype,
			initialized: true,
			timer:       nil,
		},
	}
}

//===========================================================================
// FixedInterval Declaration
//===========================================================================

// FixedInterval dispatches it's internal event type on a routine period. It
// does that by wrapping a time.Timer object, adding the additional Interval
// functionality as well as the event dispatcher functionality.
type FixedInterval struct {
	delay       time.Duration // The fixed interval to push events on
	etype       EventType     // The type of event dispatched by the timer
	initialized bool          // If the interval has been initialized
	timer       *time.Timer   // The internal timer to wrap
}

// GetDelay returns the fixed interval duration.
func (t *FixedInterval) GetDelay() time.Duration {
	return t.delay
}

// Start the interval to periodically issue events. Returns true if the
// ticker gets started, false if it's already started or uninitialized.
func (t *FixedInterval) Start() bool {
	// If the timer is already started or uninitialized return false.
	if t.Running() || !t.initialized {
		return false
	}

	// Create the new timer with the delay
	t.timer = time.AfterFunc(t.GetDelay(), t.action)
	return true
}

// dispatches the fixed interval event when the timer goes off and resets the
// timer to prepare for the next event dispatch.
func (t *FixedInterval) action() {
	if !t.Running() || t.timer.Stop() {
		// Something went wrong here, not sure how
		// TODO warn or log a warning that something went wrong
		// warn("interval event dispatched on a stopped timer")
		return
	}

	// Set the timer to nil to indicate we've stopped
	t.timer = nil

	// Dispatch the internal event
	// if err := t.Dispatch(t.etype, nil); err != nil {
	// 	t.echan <- err
	// 	return
	// }

	// Create a new timer for the next action
	t.timer = time.AfterFunc(t.GetDelay(), t.action)
}

// Stop the interval so that no more events are dispatched. Returns true if
// the call stops the interval, false if already expired or never started.
func (t *FixedInterval) Stop() bool {
	if !t.Running() {
		return false
	}

	// Stop the timer and set it to nil
	stopped := t.timer.Stop()
	t.timer = nil
	return stopped
}

// Interrupt the current interval, stopping and starting it again. Returns
// true if the interval was running and is successfully reset, false if the
// ticker was stopped or uninitialized.
func (t *FixedInterval) Interrupt() bool {
	if !t.Running() {
		return false
	}

	// Stop the timer and drain the channel
	if !t.timer.Stop() {
		<-t.timer.C
	}

	t.timer = nil
	t.timer = time.AfterFunc(t.GetDelay(), t.action)
	return true
}

// Running returns true if the timer exists and false otherwise.
func (t *FixedInterval) Running() bool {
	return t.timer != nil
}

// Register the specific event type with the callback
func (t *FixedInterval) Register(callback Callback) {
	// t.DispatcherNonTS.Register(t.etype, callback)
}

//===========================================================================
// RandomInterval Declaration
//===========================================================================

// RandomInterval dispatches its internal interval on a random period between
// the minimum and maximum delay values. Every event has a different delay.
type RandomInterval struct {
	FixedInterval
	minDelay int64
	maxDelay int64
}

// GetDelay returns a random integer in the range (minDelay, maxDelay) on
// every request for the delay, causing jitter so that no timeout occurs at
// the same time.
func (t *RandomInterval) GetDelay() time.Duration {
	t.delay = time.Duration(rand.Int63n(t.maxDelay-t.minDelay) + t.minDelay)
	return t.delay
}

// Start the interval to periodically issue events. Returns true if the
// ticker gets started, false if it's already started or uninitialized.
func (t *RandomInterval) Start() bool {
	// If the timer is already started or uninitialized return false.
	if t.Running() || !t.initialized {
		return false
	}

	// Create the new timer with the delay
	t.timer = time.AfterFunc(t.GetDelay(), t.action)
	return true
}

// dispatches the fixed interval event when the timer goes off and resets the
// timer to prepare for the next event dispatch.
func (t *RandomInterval) action() {
	if !t.Running() || t.timer.Stop() {
		// Something went wrong here, not sure how
		// TODO: log a warning or otherwise record error
		// warn("interval event dispatched on a stopped timer")
		return
	}

	// Set the timer to nil to indicate we've stopped
	t.timer = nil

	// Dispatch the internal event
	// if err := t.Dispatch(t.etype, nil); err != nil {
	// 	t.echan <- err
	// 	return
	// }

	// Create a new timer for the next action
	t.timer = time.AfterFunc(t.GetDelay(), t.action)
}

// Interrupt the current interval, stopping and starting it again. Returns
// true if the interval was running and is successfully reset, false if the
// ticker was stopped or uninitialized.
func (t *RandomInterval) Interrupt() bool {
	if !t.Running() {
		return false
	}

	// Stop the timer and drain the channel
	if !t.timer.Stop() {
		<-t.timer.C
	}

	t.timer = nil
	t.timer = time.AfterFunc(t.GetDelay(), t.action)
	return true
}
