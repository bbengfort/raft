package api

// NullEntry is an empty entry that is appended to the log.
var NullEntry = &LogEntry{Index: 0, Term: 0, Name: "", Value: nil}

// IsZero returns true if the entry is the null entry.
func (e *LogEntry) IsZero() bool {
	return e.Index == 0 && e.Term == 0 && e.Name == "" && e.Value == nil
}
