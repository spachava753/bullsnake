package runtime

import "strconv"

type stringSplitlinesMethod struct {
	value *stringValue
}

type stringSplitlinesCall struct {
	instruction int
	value       *stringValue
}

func (*stringSplitlinesMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringSplitlinesMethod) Repr() string {
	return "<built-in method splitlines of str object>"
}
func (*stringSplitlinesMethod) isValue() {}

// executeStringSplitlinesCall binds keepends and resolves its truth value
// through the ordinary frame continuation before splitting the string.
func executeStringSplitlinesCall(
	caller *frame,
	instruction int,
	base int,
	method *stringSplitlinesMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	keepEnds, exception := bindStringSplitlinesArgument(arguments, keywords)
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	call := &stringSplitlinesCall{
		instruction: instruction,
		value:       method.value,
	}
	return executeTruthWithCall(caller, keepEnds, &truthCall{
		instruction: instruction,
		original:    keepEnds,
		splitlines:  call,
	})
}

// bindStringSplitlinesArgument combines one positional-or-keyword keepends
// value while preserving duplicate and unknown-name errors.
func bindStringSplitlinesArgument(
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception) {
	if len(arguments) > 1 {
		return nil, newException(
			"TypeError",
			"splitlines() takes at most 1 argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)
	}
	keepEnds := Value(falseSingleton)
	assigned := false
	if len(arguments) == 1 {
		keepEnds = arguments[0]
		assigned = true
	}
	if keywords == nil {
		return keepEnds, nil
	}
	for _, entry := range keywords.entries {
		name, ok := entry.key.(*stringValue)
		if !ok {
			return nil, newException("TypeError", "keywords must be strings")
		}
		if name.value != "keepends" {
			return nil, newException(
				"TypeError",
				"'"+name.value+"' is an invalid keyword argument for splitlines()",
			)
		}
		if assigned {
			return nil, newException(
				"TypeError",
				"splitlines() got multiple values for argument 'keepends'",
			)
		}
		keepEnds = entry.value
		assigned = true
	}
	return keepEnds, nil
}

func finishStringSplitlines(
	frame *frame,
	call *stringSplitlinesCall,
	keepEnds bool,
) (instructionOutcome, error) {
	parts := splitStringLines(call.value.value, keepEnds)
	elements := make([]Value, len(parts))
	for index, part := range parts {
		elements[index] = &stringValue{value: part}
	}
	return pushOutcome(frame, call.instruction, &listValue{elements: elements})
}

// splitStringLines scans decoded code points, combines CRLF, and includes each
// recognized boundary only when keepEnds is true.
func splitStringLines(value string, keepEnds bool) []string {
	var parts []string
	start := 0
	for offset := 0; offset < len(value); {
		current, size, _ := decodeStringRune(value[offset:])
		if !stringLineBreak(current) {
			offset += size
			continue
		}

		breakEnd := offset + size
		if current == '\r' && breakEnd < len(value) {
			next, nextSize, _ := decodeStringRune(value[breakEnd:])
			if next == '\n' {
				breakEnd += nextSize
			}
		}
		end := offset
		if keepEnds {
			end = breakEnd
		}
		parts = append(parts, value[start:end])
		start = breakEnd
		offset = breakEnd
	}
	if start < len(value) {
		parts = append(parts, value[start:])
	}
	return parts
}

func stringLineBreak(codepoint rune) bool {
	switch codepoint {
	case '\n', '\v', '\f', '\r', 0x001c, 0x001d, 0x001e, 0x0085, 0x2028, 0x2029:
		return true
	default:
		return false
	}
}
