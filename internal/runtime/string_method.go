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

type stringEndswithMethod struct {
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

func (*stringEndswithMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringEndswithMethod) Repr() string {
	return "<built-in method endswith of str object>"
}
func (*stringEndswithMethod) isValue() {}

// executeStringAttributeLoad returns the bound native method implemented for
// one immutable string or raises the normal missing-attribute error.
func executeStringAttributeLoad(
	frame *frame,
	instruction int,
	value *stringValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "count":
		return pushOutcome(frame, instruction, &stringCountMethod{value: value})
	case "join":
		return pushOutcome(frame, instruction, &stringJoinMethod{separator: value})
	case "format":
		return pushOutcome(frame, instruction, &stringFormatMethod{value: value})
	case "lower":
		return pushOutcome(frame, instruction, &stringLowerMethod{value: value})
	case "replace":
		return pushOutcome(frame, instruction, &stringReplaceMethod{value: value})
	case "removeprefix":
		return pushOutcome(frame, instruction, &stringRemovePrefixMethod{value: value})
	case "endswith":
		return pushOutcome(frame, instruction, &stringEndswithMethod{value: value})
	case "startswith":
		return pushOutcome(frame, instruction, &stringStartswithMethod{value: value})
	case "split":
		return pushOutcome(frame, instruction, &stringSplitMethod{value: value})
	case "splitlines":
		return pushOutcome(frame, instruction, &stringSplitlinesMethod{value: value})
	case "strip":
		return pushOutcome(frame, instruction, &stringStripMethod{value: value})
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

// executeStringTailmatchCall applies positional-only candidate and slice bounds
// before checking one string or an ordered tuple of strings.
func executeStringTailmatchCall(
	caller *frame,
	instruction int,
	base int,
	value *stringValue,
	methodName string,
	suffix bool,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			methodName+"() takes no keyword arguments",
		)), nil
	}
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			methodName+"() takes at least 1 argument (0 given)",
		)), nil
	}
	if len(arguments) > 3 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			methodName+"() takes at most 3 arguments ("+
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
	length := len(stringCodepointOffsets(value.value)) - 1
	start, exception := normalizeStringTailBound(startValue, length, true)
	if exception == nil {
		var end int
		end, exception = normalizeStringTailBound(endValue, length, false)
		if exception == nil {
			matched, matchException := stringTailmatch(
				value.value,
				arguments[0],
				start,
				end,
				methodName,
				suffix,
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

// normalizeStringTailBound maps None and arbitrary-size integers into one
// tail-match bound while preserving a start beyond the string's end.
func normalizeStringTailBound(
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

// stringTailmatch validates tuple entries in source order and stops before
// inspecting later entries once one prefix or suffix matches.
func stringTailmatch(
	value string,
	candidate Value,
	start int,
	end int,
	methodName string,
	suffix bool,
) (bool, *Exception) {
	switch candidate := candidate.(type) {
	case *stringValue:
		return stringTailmatchCandidate(value, candidate.value, start, end, suffix), nil
	case *tupleValue:
		for _, alternative := range candidate.elements {
			text, ok := alternative.(*stringValue)
			if !ok {
				return false, newException(
					"TypeError",
					"tuple for "+methodName+" must only contain str, not "+
						alternative.TypeName(),
				)
			}
			if stringTailmatchCandidate(value, text.value, start, end, suffix) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, newException(
			"TypeError",
			methodName+" first arg must be str or a tuple of str, not "+
				candidate.TypeName(),
		)
	}
}

func stringTailmatchCandidate(value, candidate string, start, end int, suffix bool) bool {
	valueOffsets := stringCodepointOffsets(value)
	candidateLength := len(stringCodepointOffsets(candidate)) - 1
	valueLength := len(valueOffsets) - 1
	if start > end || start > valueLength || candidateLength > end-start {
		return false
	}
	matchStart := start
	if suffix {
		matchStart = end - candidateLength
	}
	return value[valueOffsets[matchStart]:valueOffsets[matchStart+candidateLength]] ==
		candidate
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
