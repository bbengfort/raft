package raft_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/bbengfort/raft"
)

//===========================================================================
// Mock Test Event
//===========================================================================

type testEvent struct {
	idx int
	jdx int
}

func (e *testEvent) Type() EventType {
	return EventType(99)
}

func (e *testEvent) Source() interface{} {
	return e.idx
}

func (e *testEvent) Value() interface{} {
	return e.jdx
}

var _ = Describe("Events", func() {

	It("should be able to assign mock event to EventType", func() {
		var event Event = &testEvent{} // this will fail before the assertion but is a good sanity check
		Ω(&testEvent{}).Should(BeAssignableToTypeOf(event))
	})

	It("should return unnknown as event type string repr", func() {
		event := &testEvent{}
		Ω(event.Type().String()).Should(Equal("unknown"))
	})

})
