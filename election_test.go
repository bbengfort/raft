package raft_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/bbengfort/raft"
)

var _ = Describe("Election", func() {

	var majority int
	var peers []string
	var election *Election

	Context("size 3 quorum", func() {

		BeforeEach(func() {
			majority = 2
			peers = []string{"Quintus", "Gaius", "Fabius"}
			election = NewElection(peers...)
		})

		It("should have a majority of 2", func() {
			Ω(election.Majority()).Should(Equal(majority))
		})

		It("should pass after a majority of votes", func() {
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())

			for i, peer := range peers {
				i++
				election.Vote(peer, true)
				Ω(election.Votes()).Should(Equal(i))
				if i >= majority {
					Ω(election.Passed()).Should(BeTrue())
				} else {
					Ω(election.Passed()).Should(BeFalse())
				}

			}

		})

		It("should not allow non members to vote", func() {
			election.Vote("Lucifer", true)
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())
		})

	})

	Context("size 4 quorum", func() {

		BeforeEach(func() {
			majority = 3
			peers = []string{"Quintus", "Gaius", "Fabius", "Julius"}
			election = NewElection(peers...)
		})

		It("should have a majority of 3", func() {
			Ω(election.Majority()).Should(Equal(majority))
		})

		It("should pass after a majority of votes", func() {
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())

			for i, peer := range peers {
				i++
				election.Vote(peer, true)
				Ω(election.Votes()).Should(Equal(i))
				if i >= majority {
					Ω(election.Passed()).Should(BeTrue())
				} else {
					Ω(election.Passed()).Should(BeFalse())
				}

			}

		})

		It("should not allow non members to vote", func() {
			election.Vote("Lucifer", true)
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())
		})

	})

	Context("size 5 quorum", func() {

		BeforeEach(func() {
			majority = 3
			peers = []string{"Quintus", "Gaius", "Fabius", "Julius", "Marcus"}
			election = NewElection(peers...)
		})

		It("should have a majority of 3", func() {
			Ω(election.Majority()).Should(Equal(majority))
		})

		It("should pass after a majority of votes", func() {
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())

			for i, peer := range peers {
				i++
				election.Vote(peer, true)
				Ω(election.Votes()).Should(Equal(i))
				if i >= majority {
					Ω(election.Passed()).Should(BeTrue())
				} else {
					Ω(election.Passed()).Should(BeFalse())
				}

			}

		})

		It("should not allow non members to vote", func() {
			election.Vote("Lucifer", true)
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())
		})

	})

	Context("size 6 quorum", func() {

		BeforeEach(func() {
			majority = 4
			peers = []string{"Quintus", "Gaius", "Fabius", "Julius", "Marcus", "Pontius"}
			election = NewElection(peers...)
		})

		It("should have a majority of 4", func() {
			Ω(election.Majority()).Should(Equal(majority))
		})

		It("should pass after a majority of votes", func() {
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())

			for i, peer := range peers {
				i++
				election.Vote(peer, true)
				Ω(election.Votes()).Should(Equal(i))
				if i >= majority {
					Ω(election.Passed()).Should(BeTrue())
				} else {
					Ω(election.Passed()).Should(BeFalse())
				}

			}

		})

		It("should not allow non members to vote", func() {
			election.Vote("Lucifer", true)
			Ω(election.Votes()).Should(BeZero())
			Ω(election.Passed()).Should(BeFalse())
		})

	})

})
