package runtime

import "strconv"

type stringStripMethod struct {
	value *stringValue
}

func (*stringStripMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringStripMethod) Repr() string {
	return "<built-in method strip of str object>"
}
func (*stringStripMethod) isValue() {}

// executeStringStripCall validates the optional character set and trims both
// ends at decoded code-point boundaries.
func executeStringStripCall(
	caller *frame,
	instruction int,
	base int,
	method *stringStripMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.strip() takes no keyword arguments",
		)), nil
	}
	if len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"strip expected at most 1 argument, got "+strconv.Itoa(len(arguments)),
		)), nil
	}

	characters := Value(None)
	if len(arguments) == 1 {
		characters = arguments[0]
	}
	var selected map[rune]struct{}
	if characters != None {
		text, ok := characters.(*stringValue)
		if !ok {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"TypeError",
				"strip arg must be None or str",
			)), nil
		}
		selected = stringCodepointSet(text.value)
	}

	trimmed := stripString(method.value.value, selected)
	discardCallSegment(caller, base)
	if trimmed == method.value.value {
		return pushOutcome(caller, instruction, method.value)
	}
	return pushOutcome(caller, instruction, &stringValue{value: trimmed})
}

func stringCodepointSet(value string) map[rune]struct{} {
	selected := make(map[rune]struct{})
	for offset := 0; offset < len(value); {
		codepoint, size, _ := decodeStringRune(value[offset:])
		selected[codepoint] = struct{}{}
		offset += size
	}
	return selected
}

func stripString(value string, selected map[rune]struct{}) string {
	offsets := stringCodepointOffsets(value)
	start := 0
	end := len(offsets) - 1
	for start < end && stripCodepoint(value[offsets[start]:offsets[start+1]], selected) {
		start++
	}
	for end > start && stripCodepoint(value[offsets[end-1]:offsets[end]], selected) {
		end--
	}
	return value[offsets[start]:offsets[end]]
}

func stripCodepoint(encoded string, selected map[rune]struct{}) bool {
	codepoint, _, _ := decodeStringRune(encoded)
	if selected == nil {
		return pythonWhitespace(codepoint)
	}
	_, found := selected[codepoint]
	return found
}
