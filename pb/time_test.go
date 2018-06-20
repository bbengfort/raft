package pb_test

import (
	"time"

	. "github.com/bbengfort/raft/pb"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Time", func() {

	var err error
	var ts time.Time
	var tested_at = "2017-12-05T14:34:52.072719-05:00"

	BeforeEach(func() {
		ts, err = time.Parse(time.RFC3339Nano, tested_at)
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("should correctly serialize a Time message", func() {
		msg := new(Time)
		err = msg.Set(ts)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(msg.Rfc3339).Should(Equal([]byte(tested_at)))
	})

	It("should correctly deserialize a Time message", func() {
		msg := &Time{Rfc3339: []byte(tested_at)}
		val, err := msg.Get()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(val).Should(Equal(ts))
	})

	It("should create a pb.Time message from a time.Time", func() {
		Ω(FromTime(ts)).Should(Equal(&Time{Rfc3339: []byte(tested_at)}))
	})

	It("should create a pb.Time from a null time.Time", func() {
		Ω(FromTime(time.Time{})).Should(Equal(&Time{Rfc3339: []byte("0001-01-01T00:00:00Z")}))
	})

	It("should extract a null time.Time from a null pb.Time", func() {
		msg := &Time{Rfc3339: []byte("0001-01-01T00:00:00Z")}
		val, err := msg.Get()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(val).Should(Equal(time.Time{}))
	})

	It("should extract a null time.Time from an empty string pb.Time", func() {
		msg := &Time{Rfc3339: []byte("")}
		val, err := msg.Get()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(val).Should(Equal(time.Time{}))
	})

	It("should extract a null time.Time from a nil pb.Time", func() {
		msg := &Time{Rfc3339: nil}
		val, err := msg.Get()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(val).Should(Equal(time.Time{}))
	})

})
