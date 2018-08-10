package raft_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/bbengfort/raft"
)

var _ = Describe("Config", func() {

	It("should validate a correct configuration", func() {
		conf := &Config{
			Name:     "foo",
			Seed:     42,
			Tick:     "1200ms",
			Timeout:  "300ms",
			LogLevel: 2,
			Leader:   "alpha",
			Uptime:   "15m",
			Metrics:  "metrics.json",
		}
		Ω(conf.Validate()).Should(Succeed())
	})

	It("should be valid with loaded defaults", func() {
		conf := new(Config)

		confPath, err := conf.GetPath()
		Ω(confPath).Should(BeZero())
		Ω(err).Should(HaveOccurred())

		Ω(conf.Load()).Should(Succeed())
	})

	It("should be able to parse durations", func() {
		conf := &Config{Tick: "10s", Timeout: "10s", Uptime: "10s"}

		duration, err := conf.GetTick()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(duration).Should(Equal(10 * time.Second))

		duration, err = conf.GetTimeout()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(duration).Should(Equal(10 * time.Second))

		duration, err = conf.GetUptime()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(duration).Should(Equal(10 * time.Second))
	})

})
