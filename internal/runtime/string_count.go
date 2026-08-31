package runtime

import (
	"strconv"
	"strings"
)

type stringCountMethod struct {
	value *stringValue
}

func (*stringCountMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringCountMethod) Repr() string {
	return "<built-in method count of str object>"
}
func (*stringCountMethod) isValue() {}

// executeStringCountCall applies slice-style code-point bounds before counting
// non-overlapping encoded substring matches.
func executeStringCountCall(
	caller *frame,
	instruction int,
	base int,
	method *stringCountMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"count() takes no keyword arguments",
		)), nil
	}
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"count() takes at least 1 argument (0 given)",
		)), nil
	}
	if len(arguments) > 3 {
		count := len(arguments)
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"count() takes at most 3 arguments ("+strconv.Itoa(count)+" given)",
		)), nil
	}
	substring, ok := arguments[0].(*stringValue)
	if !ok {
		typeName := arguments[0].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"must be str, not "+typeName,
		)), nil
	}

	startValue := Value(None)
	endValue := Value(None)
	if len(arguments) >= 2 {
		startValue = arguments[1]
	}
	if len(arguments) == 3 {
		endValue = arguments[2]
	}
	offsets := stringCodepointOffsets(method.value.value)
	length := len(offsets) - 1
	start, exception := normalizeStringTailBound(startValue, length, true)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	end, exception := normalizeStringTailBound(endValue, length, false)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}

	count := 0
	if start <= end && start <= length {
		if substring.value == "" {
			count = end - start + 1
		} else {
			count = strings.Count(
				method.value.value[offsets[start]:offsets[end]],
				substring.value,
			)
		}
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, integerFromInt64(int64(count)))
}
