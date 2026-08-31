package runtime

import (
	"strconv"
	"strings"
)

type stringSplitMethod struct {
	value *stringValue
}

func (*stringSplitMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringSplitMethod) Repr() string {
	return "<built-in method split of str object>"
}
func (*stringSplitMethod) isValue() {}

// executeStringSplitCall binds sep and maxsplit, applies one fixed splitting
// mode, and returns independently stored native strings.
func executeStringSplitCall(
	caller *frame,
	instruction int,
	base int,
	method *stringSplitMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	separator, maximum, exception := bindStringSplitArguments(arguments, keywords)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	maxSplit, exception := stringMaxSplit(maximum)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}

	var parts []string
	if separator == None {
		parts = splitPythonWhitespace(method.value.value, maxSplit)
	} else {
		text, ok := separator.(*stringValue)
		if !ok {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"TypeError",
				"must be str or None, not "+separator.TypeName(),
			)), nil
		}
		if text.value == "" {
			discardCallSegment(caller, base)
			return raiseOutcome(newException("ValueError", "empty separator")), nil
		}
		if maxSplit < 0 {
			parts = strings.Split(method.value.value, text.value)
		} else {
			parts = strings.SplitN(method.value.value, text.value, maxSplit+1)
		}
	}

	elements := make([]Value, len(parts))
	for index, part := range parts {
		elements[index] = &stringValue{value: part}
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &listValue{elements: elements})
}

// bindStringSplitArguments applies the two optional positional-or-keyword
// parameters and preserves duplicate and unknown-name errors.
func bindStringSplitArguments(
	arguments []Value,
	keywords *dictValue,
) (Value, Value, *Exception) {
	if len(arguments) > 2 {
		return nil, nil, newException(
			"TypeError",
			"split() takes at most 2 arguments ("+
				strconv.Itoa(len(arguments))+" given)",
		)
	}
	values := [2]Value{None, integerFromInt64(-1)}
	assigned := [2]bool{}
	for index, value := range arguments {
		values[index] = value
		assigned[index] = true
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			nameValue, ok := entry.key.(*stringValue)
			if !ok {
				return nil, nil, newException("TypeError", "keywords must be strings")
			}
			name := nameValue.value
			index := -1
			switch name {
			case "sep":
				index = 0
			case "maxsplit":
				index = 1
			default:
				return nil, nil, newException(
					"TypeError",
					"'"+name+"' is an invalid keyword argument for split()",
				)
			}
			if assigned[index] {
				return nil, nil, newException(
					"TypeError",
					"split() got multiple values for argument '"+name+"'",
				)
			}
			values[index] = entry.value
			assigned[index] = true
		}
	}
	return values[0], values[1], nil
}

func stringMaxSplit(value Value) (int, *Exception) {
	integer, ok := integerOperand(value)
	if !ok {
		return 0, newException(
			"TypeError",
			"'"+value.TypeName()+"' object cannot be interpreted as an integer",
		)
	}
	if integer.Sign() < 0 || !integer.IsInt64() {
		return -1, nil
	}
	maximum := integer.Int64()
	if int64(int(maximum)) != maximum {
		return -1, nil
	}
	return int(maximum), nil
}

// splitPythonWhitespace drops leading runs, preserves the unsplit tail once
// maxsplit is reached, and uses CPython's stable whitespace code-point set.
func splitPythonWhitespace(value string, maximum int) []string {
	offset := skipPythonWhitespace(value, 0)
	if offset == len(value) {
		return nil
	}
	if maximum == 0 {
		return []string{value[offset:]}
	}

	parts := make([]string, 0)
	splits := 0
	for offset < len(value) {
		if maximum >= 0 && splits == maximum {
			parts = append(parts, value[offset:])
			break
		}
		start := offset
		for offset < len(value) {
			codepoint, size, _ := decodeStringRune(value[offset:])
			if pythonWhitespace(codepoint) {
				break
			}
			offset += size
		}
		parts = append(parts, value[start:offset])
		offset = skipPythonWhitespace(value, offset)
		splits++
	}
	return parts
}

func skipPythonWhitespace(value string, offset int) int {
	for offset < len(value) {
		codepoint, size, _ := decodeStringRune(value[offset:])
		if !pythonWhitespace(codepoint) {
			break
		}
		offset += size
	}
	return offset
}

// pythonWhitespace implements the code-point set used by CPython 3.14 rather
// than inheriting the host Go release's Unicode whitespace classification.
func pythonWhitespace(codepoint rune) bool {
	switch {
	case codepoint >= 0x0009 && codepoint <= 0x000d:
		return true
	case codepoint >= 0x001c && codepoint <= 0x0020:
		return true
	case codepoint == 0x0085 || codepoint == 0x00a0 || codepoint == 0x1680:
		return true
	case codepoint >= 0x2000 && codepoint <= 0x200a:
		return true
	case codepoint == 0x2028 || codepoint == 0x2029 || codepoint == 0x202f:
		return true
	case codepoint == 0x205f || codepoint == 0x3000:
		return true
	default:
		return false
	}
}
