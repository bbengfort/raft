package raft_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/bbengfort/raft"
	"github.com/bbengfort/raft/pb"
)

type stateMachine struct{}

func (sm *stateMachine) CommitEntry(entry *pb.LogEntry) error { return nil }
func (sm *stateMachine) DropEntry(entry *pb.LogEntry) error   { return nil }

var _ = Describe("Log", func() {

	It("should correctly initialize empty log", func() {
		log := NewLog(&stateMachine{})

		Ω(log.LastApplied()).Should(Equal(uint64(0)))
		Ω(log.CommitIndex()).Should(Equal(uint64(0)))
		Ω(log.LastTerm()).Should(Equal(uint64(0)))
		Ω(log.CommitTerm()).Should(Equal(uint64(0)))
		Ω(log.LastEntry()).Should(Equal(pb.NullEntry))
		Ω(log.LastCommit()).Should(Equal(pb.NullEntry))
	})

	Describe("entry and accesses", func() {

		var log *Log

		BeforeEach(func() {
			log = NewLog(&stateMachine{})
		})

		Context("when log starts empty", func() {

			It("a log with entries should be as up to date as empty log", func() {
				Ω(log.AsUpToDate(3, 1)).Should(BeTrue())
			})

			It("should be able to create entries in the log", func() {

				// Create 4 entries in the current term
				for i := 0; i < 4; i++ {
					value := []byte(fmt.Sprintf("the time is now %s", time.Now()))
					_, err := log.Create("foo", value, 1)
					Ω(err).ShouldNot(HaveOccurred())
				}

				// Create the last entry
				value := []byte(fmt.Sprintf("the time is now %s", time.Now()))
				entry, err := log.Create("foo", value, 2)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(log.LastApplied()).Should(Equal(uint64(5)))
				Ω(log.CommitIndex()).Should(Equal(uint64(0)))
				Ω(log.LastTerm()).Should(Equal(uint64(2)))
				Ω(log.CommitTerm()).Should(Equal(uint64(0)))
				Ω(log.LastEntry()).Should(Equal(entry))
				Ω(log.LastCommit()).Should(Equal(pb.NullEntry))
			})

			It("should be able to append an entry in the log", func() {
				entry := &pb.LogEntry{
					Index: 1,
					Term:  42,
					Name:  "foo",
					Value: []byte(time.Now().Format(time.RFC3339Nano)),
				}

				err := log.Append(entry)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(log.LastApplied()).Should(Equal(uint64(1)))
				Ω(log.CommitIndex()).Should(Equal(uint64(0)))
				Ω(log.LastEntry()).Should(Equal(entry))
				Ω(log.LastCommit()).Should(Equal(pb.NullEntry))
			})

			It("should be able to append multiple entries in the log", func() {
				entries := []*pb.LogEntry{
					{
						Index: 1,
						Term:  42,
						Name:  "foo",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
					{
						Index: 2,
						Term:  42,
						Name:  "bar",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
					{
						Index: 3,
						Term:  42,
						Name:  "baz",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
					{
						Index: 4,
						Term:  42,
						Name:  "zop",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
				}

				err := log.Append(entries...)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(log.LastApplied()).Should(Equal(uint64(4)))
				Ω(log.CommitIndex()).Should(Equal(uint64(0)))
			})
		})

		Context("when log starts with data", func() {
			// TODO: write snapshot to disk and test log with data

			It("should be able to identify the last applied index", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.LastApplied()).Should(Equal(uint64(7)))
			})

			It("should be able to identify the commmit index", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.CommitIndex()).Should(Equal(uint64(4)))
			})

			It("should be able to get the last entry from the log", func() {
				Skip("log snapshot not implemented yet")
				entry := log.LastEntry()
				Ω(entry).ShouldNot(BeNil())
				Ω(entry.Term).ShouldNot(BeZero())
				Ω(entry.Name).ShouldNot(BeZero())
				Ω(entry.Value).ShouldNot(BeZero())
			})

			It("should be able to get the last committed entry from the log", func() {
				Skip("log snapshot not implemented yet")
				entry := log.LastCommit()
				Ω(entry).ShouldNot(BeNil())
				Ω(entry.Term).ShouldNot(BeZero())
				Ω(entry.Name).ShouldNot(BeZero())
				Ω(entry.Value).ShouldNot(BeZero())
			})

			It("should be able to get the last entry's term from the log", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.LastTerm()).Should(Equal(uint64(4)))
			})

			It("should be able to get the last committed entry's term from the log", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.CommitTerm()).Should(Equal(uint64(3)))
			})

			It("should be as up to date as other similar log", func() {
				Skip("log snapshot not implemented yet")

				// Confirm Log State
				Ω(log.CommitIndex()).Should(Equal(uint64(4)))
				Ω(log.LastApplied()).Should(Equal(uint64(7)))
				Ω(log.LastTerm()).Should(Equal(uint64(4)))

				// Index and last epoch match
				Ω(log.AsUpToDate(7, 4)).Should(BeTrue())
			})

			It("should be as up to date as a log that is farther ahead", func() {
				Skip("log snapshot not implemented yet")

				// Confirm Log State
				Ω(log.CommitIndex()).Should(Equal(uint64(4)))
				Ω(log.LastApplied()).Should(Equal(uint64(7)))
				Ω(log.LastTerm()).Should(Equal(uint64(4)))

				// Last epoch matches but other index is larger (farther ahead)
				Ω(log.AsUpToDate(9, 4)).Should(BeTrue(), "other log at higher index")

				// Other log is at an increased term
				Ω(log.AsUpToDate(9, 5)).Should(BeTrue(), "other log at higher term")

				// Other log is at an increased term but same index
				Ω(log.AsUpToDate(7, 4)).Should(BeTrue(), "other log at higher term but same index")
			})

			It("should not be as up to date as a log that is behind", func() {
				Skip("log snapshot not implemented yet")
				// Confirm Log State
				Ω(log.CommitIndex()).Should(Equal(uint64(4)))
				Ω(log.LastApplied()).Should(Equal(uint64(7)))
				Ω(log.LastTerm()).Should(Equal(uint64(4)))

				Ω(log.AsUpToDate(6, 4)).Should(BeFalse(), "other log at lower index above commit")
				Ω(log.AsUpToDate(2, 4)).Should(BeFalse(), "other log at lower index below commit")
				Ω(log.AsUpToDate(9, 3)).Should(BeFalse(), "other log at lower term")
				Ω(log.AsUpToDate(7, 3)).Should(BeFalse(), "other log at lower term but same index")
			})

			It("should be able to create entries in the log", func() {
				Skip("log snapshot not implemented yet")
				term := log.LastTerm()       // Term is same as last term
				commit := log.CommitIndex()  // Commit remains unchanged
				lastIdx := log.LastApplied() // Index gets incremented

				// Create the last entr
				value := []byte(fmt.Sprintf("the time is now %s", time.Now()))
				entry, err := log.Create("foo", value, term)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(log.LastApplied()).Should(Equal(lastIdx + 1))
				Ω(log.CommitIndex()).Should(Equal(commit))
				Ω(log.LastTerm()).Should(Equal(term))
				Ω(log.LastEntry()).Should(Equal(entry))
			})

			It("should be able to append entries in the log", func() {
				Skip("log snapshot not implemented yet")

				term := log.LastTerm()       // Epoch is same as last epoch
				commit := log.CommitIndex()  // Commit remains unchanged
				lastIdx := log.LastApplied() // Index gets incremented

				entries := []*pb.LogEntry{
					{
						Index: lastIdx + 1,
						Term:  term,
						Name:  "foo",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
					{
						Index: lastIdx + 2,
						Term:  term,
						Name:  "bar",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
					{
						Index: lastIdx + 3,
						Term:  term,
						Name:  "baz",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
					{
						Index: lastIdx + 4,
						Term:  term,
						Name:  "zop",
						Value: []byte(time.Now().Format(time.RFC3339Nano)),
					},
				}

				err := log.Append(entries...)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(log.LastApplied()).Should(Equal(lastIdx + 4))
				Ω(log.CommitIndex()).Should(Equal(commit))
				Ω(log.LastEntry().Name).Should(Equal("zop"))
			})

			It("should not append entries with a null epoch", func() {
				Skip("log snapshot not implemented yet")

				entry := &pb.LogEntry{
					Index: log.LastApplied() + 1,
					Term:  0,
					Name:  "foo",
					Value: nil,
				}

				err := log.Append(entry)
				Ω(err).Should(HaveOccurred())
			})

			It("should not append entries with an earlier term", func() {
				Skip("log snapshot not implemented yet")
				term := log.LastTerm()
				term--

				entry := &pb.LogEntry{
					Index: log.LastApplied() + 1,
					Term:  term,
					Name:  "foo",
					Value: nil,
				}

				err := log.Append(entry)
				Ω(err).Should(HaveOccurred())
			})

			It("should not append entries with an earlier index", func() {
				Skip("log snapshot not implemented yet")
				entry := &pb.LogEntry{
					Index: log.LastApplied() - 1,
					Term:  log.LastTerm(),
					Name:  "foo",
					Value: nil,
				}

				err := log.Append(entry)
				Ω(err).Should(HaveOccurred())
			})

			It("should be able to commit uncommitted entries", func() {
				Skip("log snapshot not implemented yet")

				// Ensure there are entries to commit
				Ω(log.CommitIndex()).Should(Equal(uint64(4)))
				Ω(log.LastApplied()).Should(Equal(uint64(7)))

				// This entry should be last commit when we're through
				lastEntry := log.LastEntry()

				// Commit and check that no error occurs
				Ω(log.Commit(7)).ShouldNot(HaveOccurred())
				Ω(log.CommitIndex()).Should(Equal(uint64(7)))
				Ω(log.LastCommit()).Should(Equal(lastEntry))
			})

			It("should not commit index 0", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.Commit(0)).Should(HaveOccurred())
			})

			It("should not commit an index not in the log", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.Commit(log.LastApplied() + 1)).Should(HaveOccurred())
			})

			It("should truncate uncomitted entries", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.CommitIndex()).Should(Equal(uint64(4)))
				Ω(log.LastApplied()).Should(Equal(uint64(7)))

				// This entry should be last commit when we're through
				lastCommit := log.LastCommit()

				// Truncate and check no errors occur
				Ω(log.Truncate(lastCommit.Index, lastCommit.Term)).ShouldNot(HaveOccurred())
				Ω(log.CommitIndex()).Should(Equal(uint64(4)))
				Ω(log.LastApplied()).Should(Equal(uint64(4)))
				Ω(log.LastEntry()).Should(Equal(lastCommit))

				// Callbacks should have been called on each entry dropped
				// time.Sleep(10 * time.Millisecond)
				// Ω(onDropMock.Calls()).Should(Equal(3))
			})

			It("should not truncate index 0", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.Truncate(0, 0)).Should(HaveOccurred())
			})

			It("should not truncate an index not in the log", func() {
				Skip("log snapshot not implemented yet")
				Ω(log.Truncate(log.LastApplied()+1, log.LastTerm())).Should(HaveOccurred())
			})

			It("should not truncate any committed entries", func() {
				Skip("log snapshot not implemented yet")
				lastIdx := log.LastApplied()
				commitIndex := log.CommitIndex()
				Ω(commitIndex).ShouldNot(BeZero())

				prev, _ := log.Prev(commitIndex)
				Ω(prev.Index).Should(BeNumerically("<", commitIndex))

				err := log.Truncate(prev.Index, prev.Term)
				Ω(err).Should(HaveOccurred())
				Ω(err.Error()).Should(ContainSubstring("cannot truncate index %v it is already committed", prev.Index+1))

				// Log should remain unchanged
				Ω(log.CommitIndex()).Should(Equal(commitIndex))
				Ω(log.LastApplied()).Should(Equal(lastIdx))
			})

			It("should not truncate to entry with wrong term", func() {
				Skip("log snapshot not implemented yet")
				lastIdx := log.LastApplied()
				commitIndex := log.CommitIndex()
				Ω(commitIndex).ShouldNot(BeZero())

				err := log.Truncate(commitIndex+1, 1)
				Ω(err).Should(HaveOccurred())
				Ω(err.Error()).Should(ContainSubstring("entry at index does not match epoch"))

				// Log should remain unchanged
				Ω(log.CommitIndex()).Should(Equal(commitIndex))
				Ω(log.LastApplied()).Should(Equal(lastIdx))
			})

			It("should get an entry at an index, committed or not", func() {
				Skip("log snapshot not implemented yet")
				for i := uint64(1); i <= log.LastApplied(); i++ {
					entry, err := log.Get(i)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(entry.Index).Should(Equal(uint64(i)))
				}
			})

			It("should return not found error if getting index that doesn't exist", func() {
				Skip("log snapshot not implemented yet")
				entry, err := log.Get(log.LastApplied() + 3)
				Ω(err).Should(HaveOccurred())
				Ω(entry).Should(BeNil())
			})

			It("should get an entry previous to index, committed or not", func() {
				Skip("log snapshot not implemented yet")
				entry, err := log.Get(log.CommitIndex() - 1)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(entry.Index).Should(Equal(uint64(log.CommitIndex() - 2)))

				entry, err = log.Get(log.CommitIndex() + 1)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(entry.Index).Should(Equal(uint64(log.CommitIndex())))
			})

			It("should return not found error if previous to index zero", func() {
				Skip("log snapshot not implemented yet")
				entry, err := log.Prev(0)
				Ω(err).Should(HaveOccurred())
				Ω(entry).Should(BeNil())
			})

			It("should return not found error if previous to index that doesn't exist", func() {
				Skip("log snapshot not implemented yet")
				entry, err := log.Prev(log.LastApplied() + 2)
				Ω(err).Should(HaveOccurred())
				Ω(entry).Should(BeNil())
			})

			It("should return last entry if previous to index last applied + 1", func() {
				Skip("log snapshot not implemented yet")
				entry, err := log.Prev(log.LastApplied() + 1)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(entry).Should(Equal(log.LastEntry()))
			})

			It("should return null entry previous to index 1", func() {
				Skip("log snapshot not implemented yet")
				entry, err := log.Prev(1)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(entry).Should(Equal(pb.NullEntry))
			})

			It("should return all entries after index 0 including null entry", func() {
				Skip("log snapshot not implemented yet")
				entries, err := log.After(0)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(entries).Should(HaveLen(int(log.LastApplied() - 1)))
			})

			It("should return all entries after an index, inclusive", func() {
				Skip("log snapshot not implemented yet")
				entries, err := log.After(log.CommitIndex())
				Ω(err).ShouldNot(HaveOccurred())
				Ω(entries).Should(HaveLen(int(1 + log.LastApplied() - log.CommitIndex())))
				Ω(log.LastCommit()).Should(Equal(entries[0]))
				Ω(log.LastEntry()).Should(Equal(entries[len(entries)-1]))
			})

			It("should return not found error requesting after index that doesn't exist", func() {
				Skip("log snapshot not implemented yet")
				entries, err := log.After(log.LastApplied() + 1)
				Ω(err).Should(HaveOccurred())
				Ω(entries).Should(BeNil())
			})
		})

	})

})
