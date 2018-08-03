package raft

import (
	"encoding/json"
	"os"
)

// Helper function to append json data as a one line string to the end of a
// results file without deleting the previous contents in it.
func appendJSON(path string, val interface{}) error {
	// Open the file for appending, creating it if necessary
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Marshal the JSON in one line without indents
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	// Append a newline to the data
	data = append(data, byte('\n'))

	// Append the data to the file
	_, err = f.Write(data)
	return err
}
