package runtime

import (
	"fmt"
	"strconv"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type exceptionTypeValue struct {
	name string
	base *exceptionTypeValue
}

func (*exceptionTypeValue) TypeName() string { return "type" }
func (exceptionType *exceptionTypeValue) Repr() string {
	return "<class '" + exceptionType.name + "'>"
}
func (*exceptionTypeValue) isValue() {}

func (exceptionType *exceptionTypeValue) isSubclassOf(parent *exceptionTypeValue) bool {
	for current := exceptionType; current != nil; current = current.base {
		if current == parent {
			return true
		}
	}
	return false
}

var (
	baseExceptionType       = &exceptionTypeValue{name: "BaseException"}
	exceptionType           = &exceptionTypeValue{name: "Exception", base: baseExceptionType}
	arithmeticErrorType     = &exceptionTypeValue{name: "ArithmeticError", base: exceptionType}
	assertionErrorType      = &exceptionTypeValue{name: "AssertionError", base: exceptionType}
	attributeErrorType      = &exceptionTypeValue{name: "AttributeError", base: exceptionType}
	importErrorType         = &exceptionTypeValue{name: "ImportError", base: exceptionType}
	moduleNotFoundErrorType = &exceptionTypeValue{name: "ModuleNotFoundError", base: importErrorType}
	lookupErrorType         = &exceptionTypeValue{name: "LookupError", base: exceptionType}
	indexErrorType          = &exceptionTypeValue{name: "IndexError", base: lookupErrorType}
	keyErrorType            = &exceptionTypeValue{name: "KeyError", base: lookupErrorType}
	nameErrorType           = &exceptionTypeValue{name: "NameError", base: exceptionType}
	unboundLocalErrorType   = &exceptionTypeValue{name: "UnboundLocalError", base: nameErrorType}
	runtimeErrorType        = &exceptionTypeValue{name: "RuntimeError", base: exceptionType}
	notImplementedErrorType = &exceptionTypeValue{name: "NotImplementedError", base: runtimeErrorType}
	overflowErrorType       = &exceptionTypeValue{name: "OverflowError", base: arithmeticErrorType}
	zeroDivisionErrorType   = &exceptionTypeValue{name: "ZeroDivisionError", base: arithmeticErrorType}
	typeErrorType           = &exceptionTypeValue{name: "TypeError", base: exceptionType}
	valueErrorType          = &exceptionTypeValue{name: "ValueError", base: exceptionType}
)

var builtinExceptionTypes = []*exceptionTypeValue{
	baseExceptionType,
	exceptionType,
	arithmeticErrorType,
	assertionErrorType,
	attributeErrorType,
	importErrorType,
	moduleNotFoundErrorType,
	lookupErrorType,
	indexErrorType,
	keyErrorType,
	nameErrorType,
	unboundLocalErrorType,
	runtimeErrorType,
	notImplementedErrorType,
	overflowErrorType,
	zeroDivisionErrorType,
	typeErrorType,
	valueErrorType,
}

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
	return pushOutcome(caller, instruction, newExceptionOfType(exceptionType, message))
}

// Exception is a Python exception value raised by bytecode execution.
type Exception struct {
	class             *exceptionTypeValue
	message           string
	cause             *Exception
	context           *Exception
	suppressContext   bool
	originFrame       *frame
	originInstruction int
}

func newException(typeName, message string) *Exception {
	for _, exceptionType := range builtinExceptionTypes {
		if exceptionType.name == typeName {
			return newExceptionOfType(exceptionType, message)
		}
	}
	panic("runtime: unknown exception type " + typeName)
}

func newExceptionOfType(exceptionType *exceptionTypeValue, message string) *Exception {
	return &Exception{class: exceptionType, message: message}
}

func normalizeRaisedValue(value Value, invalidMessage string) (*Exception, *Exception) {
	switch raised := value.(type) {
	case *Exception:
		return raised, nil
	case *exceptionTypeValue:
		return newExceptionOfType(raised, ""), nil
	default:
		return nil, newException("TypeError", invalidMessage)
	}
}

// attribute returns the three chain fields exposed by current exception values,
// translating absent exception links to Python None.
func (exception *Exception) attribute(name string) (Value, bool) {
	switch name {
	case "__cause__":
		if exception.cause == nil {
			return None, true
		}
		return exception.cause, true
	case "__context__":
		if exception.context == nil {
			return None, true
		}
		return exception.context, true
	case "__suppress_context__":
		if exception.suppressContext {
			return trueSingleton, true
		}
		return falseSingleton, true
	default:
		return nil, false
	}
}

// TypeName returns the Python exception class name.
func (exception *Exception) TypeName() string { return exception.class.name }

// Message returns the exception's detail text.
func (exception *Exception) Message() string { return exception.message }

// Repr returns a stable Python-like representation of the exception.
func (exception *Exception) Repr() string {
	return exception.class.name + "(" + strconv.Quote(exception.message) + ")"
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
