package runtime

import (
	"fmt"
	"strconv"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type exceptionTypeValue struct {
	name string
}

func (*exceptionTypeValue) TypeName() string { return "type" }
func (exceptionType *exceptionTypeValue) Repr() string {
	return "<class '" + exceptionType.name + "'>"
}
func (*exceptionTypeValue) isValue() {}

var assertionErrorType = &exceptionTypeValue{name: "AssertionError"}

// executeExceptionTypeCall validates an internal exception-class call, converts
// its positional arguments into the current message representation, consumes
// the caller segment, and pushes the new exception instance.
func executeExceptionTypeCall(
	caller *frame,
	instruction int,
	base int,
	exceptionType *exceptionTypeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				exceptionType.name+"() takes no keyword arguments",
			),
		}, nil
	}
	message := ""
	if len(arguments) == 1 {
		if text, ok := arguments[0].(*stringValue); ok {
			message = text.value
		} else {
			message = arguments[0].Repr()
		}
	} else if len(arguments) > 1 {
		message = (&tupleValue{elements: arguments}).Repr()
	}
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	return pushOutcome(caller, instruction, newException(exceptionType.name, message))
}

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
