package raft

import "sync"

// Buffer size to instantiate actor channels with
const actorEventBufferSize = 1024

// Actor objects listen for events (messages) and then can create more actors,
// send more messages or make local decisions that modify their own private
// state. Actors implement lockless concurrent operation (indeed, none of the
// structs in this package implement mutexes and are not thread safe
// independently). Concurrency here is based on the fact that only a single
// actor is initialized and reads event objects one at a time off of a
// buffered channel. All actor methods should be private as a result so they
// are not called from other threads.
type Actor interface {
	Listen() error        // Run the actor model listen for events and handle them
	Close() error         // Stop the actor from receiving new events (handles remaining pending events)
	Dispatch(Event) error // Outside callers can dispatch events to the actor
	Handle(Event) error   // Handler method for each event in sequence
}

//===========================================================================
// Non-Blocking Actor
//===========================================================================

// NewActor returns a new simple actor that passes events one at a time to the
// callback function specified by looping on an internal buffered channel so
// that event dispatchers are not blocked.
func NewActor(callback Callback) Actor {
	return &actor{
		handler: callback,
		events:  make(chan Event, actorEventBufferSize),
	}
}

// A simple implementation of an actor object that can be embedded into other
// objects so they only have to implement the Handle method to meet the
// interface requirements.
type actor struct {
	handler Callback
	events  chan Event
}

// Listen for events, handling them with the default callback handler. If the
// callback returns an error, then Listen will return with that error. If the
// actor is closed externally, then Listen will finish all remaining events
// and return nil.
func (a *actor) Listen() error {
	// Continue reading events off the channel until its closed
	for event := range a.events {
		if err := a.Handle(event); err != nil {
			return err
		}
	}

	return nil
}

// Close the actor by shutting down the events channel, allowing the listener
// to complete all remaining events then stop listening gracefully.
func (a *actor) Close() error {
	close(a.events)
	return nil
}

// Dispatch an event on the actor for the listener to handle.
func (a *actor) Dispatch(e Event) error {
	a.events <- e
	return nil
}

// Handle each event by passing it to the callback function.
func (a *actor) Handle(e Event) error {
	return a.handler(e)
}

//===========================================================================
// Blocking Actor
//===========================================================================

// NewLocker returns a new simple actor that passes events one at a time to the
// callback function specified by locking on every dispatch call so that event
// dispatchers must wait until the event is successfully dispatched.
func NewLocker(callback Callback) Actor {
	return &locker{
		handler: callback,
		done:    make(chan error),
	}
}

type locker struct {
	sync.RWMutex
	handler Callback
	done    chan error
}

func (a *locker) Listen() error {
	return <-a.done
}

func (a *locker) Close() error {
	a.Lock()
	defer a.Unlock()
	a.done <- nil
	return nil
}

func (a *locker) Dispatch(e Event) error {
	a.Lock()
	defer a.Unlock()
	if err := a.Handle(e); err != nil {
		a.done <- err
	}
	return nil
}

func (a *locker) Handle(e Event) error {
	return a.handler(e)
}
