package runtime

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type exceptionTypeValue struct {
	name           string
	base           *exceptionTypeValue
	additionalBase *exceptionTypeValue
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
		if current.additionalBase != nil && current.additionalBase.isSubclassOf(parent) {
			return true
		}
	}
	return false
}

var (
	baseExceptionType       = &exceptionTypeValue{name: "BaseException"}
	generatorExitType       = &exceptionTypeValue{name: "GeneratorExit", base: baseExceptionType}
	exceptionType           = &exceptionTypeValue{name: "Exception", base: baseExceptionType}
	baseExceptionGroupType  = &exceptionTypeValue{name: "BaseExceptionGroup", base: baseExceptionType}
	exceptionGroupType      = &exceptionTypeValue{name: "ExceptionGroup", base: baseExceptionGroupType, additionalBase: exceptionType}
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
	stopIterationType       = &exceptionTypeValue{name: "StopIteration", base: exceptionType}
	stopAsyncIterationType  = &exceptionTypeValue{name: "StopAsyncIteration", base: exceptionType}
	overflowErrorType       = &exceptionTypeValue{name: "OverflowError", base: arithmeticErrorType}
	zeroDivisionErrorType   = &exceptionTypeValue{name: "ZeroDivisionError", base: arithmeticErrorType}
	typeErrorType           = &exceptionTypeValue{name: "TypeError", base: exceptionType}
	valueErrorType          = &exceptionTypeValue{name: "ValueError", base: exceptionType}
)

var builtinExceptionTypes = []*exceptionTypeValue{
	baseExceptionType,
	generatorExitType,
	exceptionType,
	baseExceptionGroupType,
	exceptionGroupType,
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
	stopIterationType,
	stopAsyncIterationType,
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
	if isExceptionGroupType(exceptionType) {
		return executeExceptionGroupTypeCall(
			caller,
			instruction,
			base,
			exceptionType,
			nil,
			arguments,
			keywords,
		)
	}
	if keywords != nil && len(keywords.entries) != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				exceptionType.name+"() takes no keyword arguments",
			),
		}, nil
	}
	message := exceptionMessage(arguments)
	exception := newExceptionOfType(exceptionType, message)
	if exceptionType.isSubclassOf(stopIterationType) {
		exception.stopIterationValue = None
		if len(arguments) != 0 {
			exception.stopIterationValue = arguments[0]
		}
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, exception)
}

// executeUserExceptionTypeCall rejects custom initializers, delegates group
// subclasses, and constructs ordinary user exceptions from positional values.
func executeUserExceptionTypeCall(
	caller *frame,
	instruction int,
	base int,
	class *typeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if _, hasInitializer := class.lookup("__init__"); hasInitializer {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"custom exception initializers are not supported",
			),
		}, nil
	}
	if exceptionBase := class.builtinExceptionBase(); isExceptionGroupType(exceptionBase) {
		return executeExceptionGroupTypeCall(
			caller,
			instruction,
			base,
			exceptionBase,
			class,
			arguments,
			keywords,
		)
	}
	if keywords != nil && len(keywords.entries) != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				class.name+"() takes no keyword arguments",
			),
		}, nil
	}
	message := exceptionMessage(arguments)
	exception := newUserException(class, message)
	if class.builtinExceptionBase().isSubclassOf(stopIterationType) {
		exception.stopIterationValue = None
		if len(arguments) != 0 {
			exception.stopIterationValue = arguments[0]
		}
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, exception)
}

func exceptionMessage(arguments []Value) string {
	if len(arguments) == 1 {
		if text, ok := arguments[0].(*stringValue); ok {
			return text.value
		}
		return arguments[0].Repr()
	}
	if len(arguments) > 1 {
		return (&tupleValue{elements: arguments}).Repr()
	}
	return ""
}

type tracebackEntry struct {
	frame       *frame
	instruction int
}

// TracebackFrame describes one Python frame crossed while an exception unwinds.
type TracebackFrame struct {
	Filename      string
	Name          string
	QualifiedName string
	Span          lexer.Span
}

// Exception is a Python exception value raised by bytecode execution.
type Exception struct {
	class              *exceptionTypeValue
	userClass          *typeValue
	message            string
	group              *tupleValue
	stopIterationValue Value
	cause              *Exception
	context            *Exception
	suppressContext    bool
	originFrame        *frame
	originInstruction  int
	traceback          []tracebackEntry
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

func newStopIteration(value Value) *Exception {
	message := ""
	if value != None {
		message = exceptionMessage([]Value{value})
	}
	return &Exception{
		class:              stopIterationType,
		message:            message,
		stopIterationValue: value,
	}
}

func isStopIteration(exception *Exception) bool {
	return exception != nil && exception.class != nil &&
		exception.class.isSubclassOf(stopIterationType)
}

func isStopAsyncIteration(exception *Exception) bool {
	return exception != nil && exception.class != nil &&
		exception.class.isSubclassOf(stopAsyncIterationType)
}

func newUserException(class *typeValue, message string) *Exception {
	return &Exception{
		class:     class.builtinExceptionBase(),
		userClass: class,
		message:   message,
	}
}

// normalizeRaisedValue accepts exception instances or instantiates supported
// built-in and user exception classes, rejecting every other raised value.
func normalizeRaisedValue(value Value, invalidMessage string) (*Exception, *Exception) {
	switch raised := value.(type) {
	case *Exception:
		return raised, nil
	case *exceptionTypeValue:
		if isExceptionGroupType(raised) {
			return nil, exceptionGroupArityError(0)
		}
		return newExceptionOfType(raised, ""), nil
	case *typeValue:
		if !raised.isExceptionClass() {
			return nil, newException("TypeError", invalidMessage)
		}
		if _, hasInitializer := raised.lookup("__init__"); hasInitializer {
			return nil, newException(
				"TypeError",
				"custom exception initializers are not supported",
			)
		}
		if isExceptionGroupType(raised.builtinExceptionBase()) {
			return nil, exceptionGroupArityError(0)
		}
		return newUserException(raised, ""), nil
	default:
		return nil, newException("TypeError", invalidMessage)
	}
}

// chainContext links the active handled exception while removing a back-link
// that would make the new context chain cyclic.
func (exception *Exception) chainContext(context *Exception) {
	if context == nil || context == exception {
		return
	}
	seen := make(map[*Exception]struct{})
	for current := context; current != nil; current = current.context {
		if _, exists := seen[current]; exists {
			break
		}
		seen[current] = struct{}{}
		if current.context == exception {
			current.context = nil
			break
		}
	}
	exception.context = context
}

func (exception *Exception) tracebackFrames() []TracebackFrame {
	frames := make([]TracebackFrame, len(exception.traceback))
	for index, entry := range exception.traceback {
		code := entry.frame.code.code
		frames[len(frames)-1-index] = TracebackFrame{
			Filename:      code.Filename(),
			Name:          code.Name(),
			QualifiedName: code.QualifiedName(),
			Span:          entry.frame.position(entry.instruction),
		}
	}
	return frames
}

// attribute returns the chain fields shared by all exceptions and the immutable
// message and child tuple held by an exception group.
func (exception *Exception) attribute(name string) (Value, bool) {
	switch name {
	case "value":
		if exception.class == nil || !exception.class.isSubclassOf(stopIterationType) {
			return nil, false
		}
		if exception.stopIterationValue == nil {
			return None, true
		}
		return exception.stopIterationValue, true
	case "message":
		if exception.group == nil {
			return nil, false
		}
		return &stringValue{value: exception.message}, true
	case "exceptions":
		if exception.group == nil {
			return nil, false
		}
		return exception.group, true
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
func (exception *Exception) TypeName() string {
	if exception.userClass != nil {
		return exception.userClass.name
	}
	return exception.class.name
}

// Message returns the exception's detail text.
func (exception *Exception) Message() string {
	if exception.group != nil {
		return fmt.Sprintf(
			"%s (%d sub-exceptions)",
			exception.message,
			len(exception.group.elements),
		)
	}
	return exception.message
}

// Repr returns a stable Python-like representation of the exception.
func (exception *Exception) Repr() string {
	if exception.group != nil {
		children := (&listValue{elements: exception.group.elements}).Repr()
		return exception.TypeName() + "(" + strconv.Quote(exception.message) + ", " + children + ")"
	}
	return exception.TypeName() + "(" + strconv.Quote(exception.message) + ")"
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
	traceback []TracebackFrame
}

// Exception returns the raised Python value.
func (raised *UncaughtException) Exception() *Exception { return raised.exception }

// Traceback returns an outermost-first copy of the Python frame chain.
func (raised *UncaughtException) Traceback() []TracebackFrame {
	return slices.Clone(raised.traceback)
}

// Backtrace formats the complete Python frame chain and final exception.
func (raised *UncaughtException) Backtrace() string {
	var output strings.Builder
	if len(raised.traceback) != 0 {
		output.WriteString("Traceback (most recent call last):\n")
	}
	for _, frame := range raised.traceback {
		fmt.Fprintf(
			&output,
			"  File %q, line %d, in %s\n",
			frame.Filename,
			frame.Span.Start.Line,
			frame.Name,
		)
	}
	fmt.Fprintf(
		&output,
		"%s: %s",
		raised.exception.TypeName(),
		raised.exception.Message(),
	)
	return output.String()
}

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
