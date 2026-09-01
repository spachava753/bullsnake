package runtime

import "strconv"

type exceptionWithTracebackMethod struct {
	exception *Exception
}

func (*exceptionWithTracebackMethod) TypeName() string {
	return "builtin_function_or_method"
}

func (method *exceptionWithTracebackMethod) Repr() string {
	return "<built-in method with_traceback of " + method.exception.TypeName() + " object>"
}

func (*exceptionWithTracebackMethod) isValue() {}

func executeExceptionWithTracebackCall(
	caller *frame,
	instruction int,
	base int,
	method *exceptionWithTracebackMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			method.exception.TypeName()+".with_traceback() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"BaseException.with_traceback() takes exactly one argument ("+
				count+" given)",
		)), nil
	}
	if arguments[0] != None {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"__traceback__ must be a traceback or None",
		)), nil
	}
	method.exception.traceback = nil
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, method.exception)
}

var _ Value = (*exceptionWithTracebackMethod)(nil)
