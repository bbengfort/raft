package raft_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	. "github.com/bbengfort/raft"
)

var _ = Describe("Config", func() {

	It("should validate a correct configuration", func() {
		conf := &Config{
			Name:      "foo",
			Seed:      42,
			Tick:      "1200ms",
			Timeout:   "300ms",
			Aggregate: true,
			LogLevel:  "debug",
			Leader:    "alpha",
			Uptime:    "15m",
			Metrics:   "metrics.json",
		}
		Ω(conf.Validate()).Should(Succeed())
	})

	It("should be valid with loaded defaults", func() {
		conf := new(Config)

		confPath, err := conf.GetPath()
		Ω(confPath).Should(BeZero())
		Ω(err).Should(HaveOccurred())

		Ω(conf.Load()).Should(Succeed())

		// Validate configuration defaults
		Ω(conf.Tick).Should(Equal("1s"))
		Ω(conf.Timeout).Should(Equal("500ms"))
		Ω(conf.Aggregate).Should(BeTrue())
		Ω(conf.GetLogLevel()).Should(Equal(zerolog.InfoLevel))

		// Validate non configurations
		Ω(conf.Name).Should(BeZero())
		Ω(conf.Seed).Should(BeZero())
		Ω(conf.Leader).Should(BeZero())
		Ω(conf.Peers).Should(BeZero())
		Ω(conf.Uptime).Should(BeZero())
		Ω(conf.Metrics).Should(BeZero())
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
