package raft_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/bbengfort/raft"
)

var _ = Describe("Interval", func() {

	var actor Actor
	var events []Event
	var interval Interval

	BeforeEach(func() {
		events = make([]Event, 0)
		actor = NewLocker(func(e Event) error {
			events = append(events, e)
			return nil
		})
	})

	Describe("FixedInterval", func() {

		BeforeEach(func() {
			interval = NewFixedInterval(actor, 5*time.Millisecond, MessageEvent)
		})

		It("should return the same interval on GetDelay", func() {
			for i := 0; i < 100; i++ {
				Ω(interval.GetDelay()).Should(Equal(5 * time.Millisecond))
			}
		})

		It("should be able to be started and stopped multiple times", func() {
			Ω(interval.Start()).Should(BeTrue())
			Ω(interval.Start()).Should(BeFalse())

			Ω(interval.Stop()).Should(BeTrue())
			Ω(interval.Stop()).Should(BeFalse())
		})

	})

	Describe("RandomInterval", func() {
		BeforeEach(func() {
			interval = NewRandomInterval(actor, 4*time.Millisecond, 6*time.Millisecond, MessageEvent)
		})

		It("should return a random interval, end inclusive on GetDelay", func() {
			prev := time.Duration(0)
			for i := 0; i < 100; i++ {
				delay := interval.GetDelay()
				Ω(delay).ShouldNot(Equal(prev))
				Ω(delay).Should(BeNumerically(">=", 4*time.Millisecond))
				Ω(delay).Should(BeNumerically("<=", 6*time.Millisecond))
				prev = delay
			}
		})

		It("should be able to be started and stopped multiple times", func() {
			Ω(interval.Start()).Should(BeTrue())
			Ω(interval.Start()).Should(BeFalse())

			Ω(interval.Stop()).Should(BeTrue())
			Ω(interval.Stop()).Should(BeFalse())
		})
	})

})
