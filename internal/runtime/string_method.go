package runtime

import (
	"math/big"
	"strconv"
	"strings"
)

type stringJoinMethod struct {
	separator *stringValue
}

type stringStartswithMethod struct {
	value *stringValue
}

func (*stringJoinMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringJoinMethod) Repr() string {
	return "<built-in method join of str object>"
}
func (*stringJoinMethod) isValue() {}

func (*stringStartswithMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringStartswithMethod) Repr() string {
	return "<built-in method startswith of str object>"
}
func (*stringStartswithMethod) isValue() {}

func executeStringAttributeLoad(
	frame *frame,
	instruction int,
	value *stringValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "join":
		return pushOutcome(frame, instruction, &stringJoinMethod{separator: value})
	case "startswith":
		return pushOutcome(frame, instruction, &stringStartswithMethod{value: value})
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'str' object has no attribute '"+name+"'",
		)), nil
	}
}

// executeStringJoinCall validates the bound method call and delegates iterable
// collection to the existing resumable constructor path.
func executeStringJoinCall(
	caller *frame,
	instruction int,
	base int,
	method *stringJoinMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.join() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.join() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	call := &collectionConstructorCall{
		instruction: instruction,
		kind:        collectionStringJoin,
		iterable:    arguments[0],
		separator:   method.separator,
	}
	discardCallSegment(caller, base)
	return startCollectionConstructor(caller, call)
}

// executeStringStartswithCall applies positional-only prefix and slice bounds
// before checking one string or an ordered tuple of strings.
func executeStringStartswithCall(
	caller *frame,
	instruction int,
	base int,
	method *stringStartswithMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"startswith() takes no keyword arguments",
		)), nil
	}
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"startswith() takes at least 1 argument (0 given)",
		)), nil
	}
	if len(arguments) > 3 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"startswith() takes at most 3 arguments ("+
				strconv.Itoa(len(arguments))+" given)",
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
	length := len(stringCodepointOffsets(method.value.value)) - 1
	start, exception := normalizeStartswithBound(startValue, length, true)
	if exception == nil {
		var end int
		end, exception = normalizeStartswithBound(endValue, length, false)
		if exception == nil {
			matched, matchException := stringStartswith(
				method.value.value,
				arguments[0],
				start,
				end,
			)
			exception = matchException
			if exception == nil {
				discardCallSegment(caller, base)
				return pushOutcome(caller, instruction, booleanValue(matched))
			}
		}
	}
	discardCallSegment(caller, base)
	return raiseOutcome(exception), nil
}

// normalizeStartswithBound maps None and arbitrary-size integers into one
// tail-match bound while preserving a start beyond the string's end.
func normalizeStartswithBound(
	value Value,
	length int,
	start bool,
) (int, *Exception) {
	if value == None {
		if start {
			return 0, nil
		}
		return length, nil
	}
	index, ok := integerOperand(value)
	if !ok {
		return 0, sliceIndexTypeError()
	}
	var limit big.Int
	limit.SetInt64(int64(length))
	if index.Sign() < 0 {
		index.Add(&index, &limit)
		if index.Sign() < 0 {
			return 0, nil
		}
	}
	if index.Cmp(&limit) > 0 {
		if start {
			return length + 1, nil
		}
		return length, nil
	}
	return int(index.Int64()), nil
}

// stringStartswith validates one prefix or tuple entries in source order and
// stops before inspecting later tuple entries once one matches.
func stringStartswith(
	value string,
	prefix Value,
	start int,
	end int,
) (bool, *Exception) {
	switch prefix := prefix.(type) {
	case *stringValue:
		return stringStartswithCandidate(value, prefix.value, start, end), nil
	case *tupleValue:
		for _, candidate := range prefix.elements {
			text, ok := candidate.(*stringValue)
			if !ok {
				return false, newException(
					"TypeError",
					"tuple for startswith must only contain str, not "+
						candidate.TypeName(),
				)
			}
			if stringStartswithCandidate(value, text.value, start, end) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, newException(
			"TypeError",
			"startswith first arg must be str or a tuple of str, not "+
				prefix.TypeName(),
		)
	}
}

func stringStartswithCandidate(value, prefix string, start, end int) bool {
	valueOffsets := stringCodepointOffsets(value)
	prefixLength := len(stringCodepointOffsets(prefix)) - 1
	valueLength := len(valueOffsets) - 1
	if start > end || start > valueLength || prefixLength > end-start {
		return false
	}
	return value[valueOffsets[start]:valueOffsets[start+prefixLength]] == prefix
}

// finishStringJoin checks every materialized item before concatenating the
// retained WTF-8-compatible string bytes in source order.
func finishStringJoin(
	frame *frame,
	call *collectionConstructorCall,
	elements []Value,
) (instructionOutcome, error) {
	parts := make([]string, len(elements))
	for index, element := range elements {
		text, ok := element.(*stringValue)
		if !ok {
			return raiseOutcome(newException(
				"TypeError",
				"sequence item "+strconv.Itoa(index)+
					": expected str instance, "+element.TypeName()+" found",
			)), nil
		}
		parts[index] = text.value
	}
	return pushOutcome(frame, call.instruction, &stringValue{
		value: strings.Join(parts, call.separator.value),
	})
}
