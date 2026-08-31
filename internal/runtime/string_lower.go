package runtime

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type stringLowerMethod struct {
	value *stringValue
}

func (*stringLowerMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringLowerMethod) Repr() string {
	return "<built-in method lower of str object>"
}
func (*stringLowerMethod) isValue() {}

// executeStringLowerCall validates the no-argument method and applies Unicode
// lowercase mappings without rewriting retained lone-surrogate bytes.
func executeStringLowerCall(
	caller *frame,
	instruction int,
	base int,
	method *stringLowerMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.lower() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.lower() takes no arguments ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}

	lowered := lowerString(method.value.value)
	discardCallSegment(caller, base)
	if lowered == method.value.value {
		return pushOutcome(caller, instruction, method.value)
	}
	return pushOutcome(caller, instruction, &stringValue{value: lowered})
}

func lowerString(value string) string {
	lower := cases.Lower(language.Und)
	var builder strings.Builder
	start := 0
	for offset := 0; offset < len(value); {
		_, size, valid := decodeStringRune(value[offset:])
		if valid && utf8.ValidString(value[offset:offset+size]) {
			offset += size
			continue
		}
		builder.WriteString(lower.String(value[start:offset]))
		builder.WriteString(value[offset : offset+size])
		offset += size
		start = offset
	}
	if start == 0 {
		return lower.String(value)
	}
	builder.WriteString(lower.String(value[start:]))
	return builder.String()
}
