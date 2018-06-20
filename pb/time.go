package pb

import "time"

// FromTime creates a pb.Time message from a time.Time timestamp.
func FromTime(ts time.Time) *Time {
	msg := new(Time)
	msg.Set(ts)
	return msg
}

// Get the time.Time from the protobuf message.
func (t *Time) Get() (time.Time, error) {
	if t.Rfc3339 == nil || len(t.Rfc3339) == 0 {
		return time.Time{}, nil
	}

	ts := new(time.Time)
	err := ts.UnmarshalText(t.Rfc3339)
	return *ts, err
}

// Set the time.Time from the protobuf message.
func (t *Time) Set(ts time.Time) (err error) {
	t.Rfc3339, err = ts.MarshalText()
	return err
}
