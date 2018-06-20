package raft_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/bbengfort/raft"
)

var _ = Describe("Actor", func() {

	ops := 1024
	var actor Actor
	var events []*testEvent

	Context("channel actor", func() {

		BeforeEach(func() {
			events = make([]*testEvent, 0)
			actor = NewActor(func(e Event) error {
				events = append(events, e.(*testEvent))
				return nil
			})
		})

		It("should concurrently append to the events slice, one event at a time", func() {

			// Dispatch a number of test event generators in its own routine
			go func() {
				var wg sync.WaitGroup

				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						for j := 0; j < 10; j++ {
							time.Sleep(1 * time.Millisecond)
							actor.Dispatch(&testEvent{i, j})
						}
					}(i)
				}

				// Join on the event dispatchers, then close
				wg.Wait()
				Ω(actor.Close()).Should(Succeed())
			}()

			// Close and wait for actor to finish
			Ω(actor.Listen()).Should(Succeed())
			Ω(events).Should(HaveLen(100))

		})

		Measure("channel throughput", func(b Benchmarker) {
			go func() {
				var wg sync.WaitGroup

				for i := 0; i < ops; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						actor.Dispatch(&testEvent{i, 0})
					}(i)
				}

				wg.Wait()
				actor.Close()
			}()

			// Dispatch a number of test event generators
			start := time.Now()
			actor.Listen()
			delta := time.Since(start)
			Ω(events).Should(HaveLen(ops))
			b.RecordValue("throughput (ops/sec)", float64(ops)/delta.Seconds())

		}, 12)
	})

	Context("mutex actor", func() {

		BeforeEach(func() {
			events = make([]*testEvent, 0)
			actor = NewLocker(func(e Event) error {
				events = append(events, e.(*testEvent))
				return nil
			})
		})

		It("should concurrently append to the events slice, one event at a time", func() {

			// Dispatch a number of test event generators in its own routine
			go func() {
				var wg sync.WaitGroup

				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						for j := 0; j < 10; j++ {
							time.Sleep(1 * time.Millisecond)
							actor.Dispatch(&testEvent{i, j})
						}
					}(i)
				}

				// Join on the event dispatchers, then close
				wg.Wait()
				Ω(actor.Close()).Should(Succeed())
			}()

			// Close and wait for actor to finish
			Ω(actor.Listen()).Should(Succeed())
			Ω(events).Should(HaveLen(100))

		})

		Measure("mutex throughput", func(b Benchmarker) {
			go func() {
				var wg sync.WaitGroup

				for i := 0; i < ops; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						actor.Dispatch(&testEvent{i, 0})
					}(i)
				}

				wg.Wait()
				actor.Close()
			}()

			// Dispatch a number of test event generators
			start := time.Now()
			actor.Listen()
			delta := time.Since(start)
			Ω(events).Should(HaveLen(ops))
			b.RecordValue("throughput (ops/sec)", float64(ops)/delta.Seconds())

		}, 12)
	})
})
