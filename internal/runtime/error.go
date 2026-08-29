package runtime

import (
	"fmt"
	"strconv"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// Exception is a Python exception value raised by bytecode execution.
type Exception struct {
	typeName string
	message  string
}

func newException(typeName, message string) *Exception {
	return &Exception{typeName: typeName, message: message}
}

// TypeName returns the Python exception class name.
func (exception *Exception) TypeName() string { return exception.typeName }

// Message returns the exception's detail text.
func (exception *Exception) Message() string { return exception.message }

// Repr returns a stable Python-like representation of the exception.
func (exception *Exception) Repr() string {
	return exception.typeName + "(" + strconv.Quote(exception.message) + ")"
}

func (*Exception) isValue() {}

// BytecodeError reports invalid or unsupported code before it executes.
type BytecodeError struct {
	Filename    string
	Instruction int
	Span        lexer.Span
	Message     string
}

// Error formats the bytecode failure with its source position when available.
func (failure *BytecodeError) Error() string {
	location := failure.Filename
	if failure.Span.Start.Line > 0 {
		location = fmt.Sprintf(
			"%s:%d:%d",
			location,
			failure.Span.Start.Line,
			failure.Span.Start.Column+1,
		)
	}
	instruction := ""
	if failure.Instruction >= 0 {
		instruction = fmt.Sprintf(" at instruction %d", failure.Instruction)
	}
	if location == "" {
		return fmt.Sprintf("bytecode error%s: %s", instruction, failure.Message)
	}
	return fmt.Sprintf("%s: bytecode error%s: %s", location, instruction, failure.Message)
}

// UncaughtException carries a Python exception across the Go host boundary.
type UncaughtException struct {
	exception *Exception
	filename  string
	span      lexer.Span
}

// Exception returns the raised Python value.
func (raised *UncaughtException) Exception() *Exception { return raised.exception }

// Error formats the uncaught exception at the instruction that raised it.
func (raised *UncaughtException) Error() string {
	location := raised.filename
	if raised.span.Start.Line > 0 {
		location = fmt.Sprintf(
			"%s:%d:%d",
			location,
			raised.span.Start.Line,
			raised.span.Start.Column+1,
		)
	}
	message := raised.exception.TypeName() + ": " + raised.exception.Message()
	if location == "" {
		return message
	}
	return location + ": " + message
}
