// Package utils contains generic functions used by different parts of the program
package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Cond is like a ternary operator, but without the short-circuiting
func Cond[T any](isTrue bool, ifTrue, ifFalse T) T {
	if isTrue {
		return ifTrue
	}
	return ifFalse
}

// IgnoreError ignores an error and returns the first parameter
func IgnoreError[T any](param1 T, _ error) T { return param1 }

// ExecCommand executes a command and returns a friendly string of the execution status. Also returns if the command failed.
func ExecCommand(commandName, command string, params ...string) (string, bool) {
	outputString, err := ExecCommandRaw(command, params...)
	if err != nil {
		return fmt.Sprintf("Failed to execute %s (%v): %s", commandName, err, outputString), false
	}
	return fmt.Sprintf("%s executed: %s", commandName, outputString), true
}

// ExecCommandRaw executes a command and returns the output, and error if there is one
func ExecCommandRaw(command string, params ...string) (string, error) {
	output, err := exec.Command(command, params...).CombinedOutput()
	return strings.TrimRight(string(output), "\n"), err
}

// CanAccessFile returns if a file is accessible
func CanAccessFile(path string) bool {
	if info, err := os.Stat(path); err != nil {
		return false
	} else {
		return !info.IsDir()
	}
}

var customLoggerObj = log.New(os.Stdout, "", 0)
var errorLoggerObj = log.New(os.Stderr, "", 0)

// CustomLogger is a logger that uses a given start time and also outputs the seconds difference between now and the startTime
func CustomLogger(startTime time.Time, format string, args ...interface{}) {
	customLoggerObj.Printf(
		"%s/%05.2f %s\n",
		startTime.Format("2006/01/02 15:04:05"),
		time.Since(startTime).Seconds(),
		fmt.Sprintf(format, args...),
	)
}

// PrintError calls log.Printf(fmt+"\n", ...) to stderr
func PrintError(format string, args ...interface{}) {
	errorLoggerObj.Printf(
		"%s %s\n",
		time.Now().Format("2006/01/02 15:04:05"),
		fmt.Sprintf(format, args...),
	)
}
