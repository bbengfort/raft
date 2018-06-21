package raft

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// Levels for implementing the debug and trace message functionality.
const (
	LogTrace uint8 = iota
	LogDebug
	LogInfo
	LogCaution
	LogStatus
	LogWarn
	LogSilent
)

// CautionThreshold for issuing caution logs after accumulating cautions.
const CautionThreshold = 80

// These variables are initialized in init()
var (
	logLevel        = LogCaution
	logger          *log.Logger
	cautionCounter  *counter
	logLevelStrings = [...]string{
		"trace", "debug", "info", "caution", "status", "warn", "silent",
	}
)

type counter struct {
	sync.Mutex
	counts map[string]uint
}

func (c *counter) init() {
	c.counts = make(map[string]uint)
}

//===========================================================================
// Interact with debug output
//===========================================================================

// LogLevel returns a string representation of the current level
func LogLevel() string {
	return logLevelStrings[logLevel]
}

// SetLogLevel modifies the log level for messages at runtime. Ensures that
// the highest level that can be set is the trace level.
func SetLogLevel(level uint8) {
	if level > LogSilent {
		level = LogSilent
	}

	logLevel = level
}

// SetLogger sets the logger for writing output to. Can set to a noplog to
// remove all log messages (or set the log level to silent).
func SetLogger(l *log.Logger) {
	logger = l
}

//===========================================================================
// Debugging output functions
//===========================================================================

// Print to the standard logger at the specified level. Arguments are handled
// in the manner of log.Printf, but a newline is appended.
func print(level uint8, msg string, a ...interface{}) {
	if level >= logLevel {
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}

		logger.Printf(msg, a...)
	}
}

// Prints to the standard logger if level is warn or greater; arguments are
// handled in the manner of log.Printf, but a newline is appended.
func warn(msg string, a ...interface{}) {
	print(LogWarn, msg, a...)
}

// Helper function to simply warn about an error received.
func warne(err error) {
	warn(err.Error())
}

// Prints to the standard logger if level is status or greater; arguments are
// handled in the manner of log.Printf, but a newline is appended.
func status(msg string, a ...interface{}) {
	print(LogStatus, msg, a...)
}

// Caution messages only log if the number of the same caution messages is
// greater than the CautionThreshold, reducing the number of log messages
// in the system but still reporting valuable information.
//
// NOTE: take care with string formatting individual messages, this could
// lead to a very full caution counter that is taking up memory.
func caution(msg string, a ...interface{}) {
	if logLevel < LogCaution {
		// Don't waste memory if the log level is set below caution.
		return
	}

	cautionCounter.Lock()
	defer cautionCounter.Unlock()

	msg = fmt.Sprintf(msg, a...)
	cautionCounter.counts[msg]++

	if cautionCounter.counts[msg] >= CautionThreshold {
		print(LogCaution, msg)
		delete(cautionCounter.counts, msg)
	}
}

// Prints to the standard logger if level is info or greater; arguments are
// handled in the manner of log.Printf, but a newline is appended.
func info(msg string, a ...interface{}) {
	print(LogInfo, msg, a...)
}

// Prints to the standard logger if level is debug or greater; arguments are
// handled in the manner of log.Printf, but a newline is appended.
func debug(msg string, a ...interface{}) {
	print(LogDebug, msg, a...)
}

// Prints to the standard logger if level is trace or greater; arguments are
// handled in the manner of log.Printf, but a newline is appended.
func trace(msg string, a ...interface{}) {
	print(LogTrace, msg, a...)
}
