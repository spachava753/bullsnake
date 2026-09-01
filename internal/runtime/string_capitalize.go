package runtime

import (
	"strconv"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type stringCapitalizeMethod struct {
	value *stringValue
}

func (*stringCapitalizeMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringCapitalizeMethod) Repr() string {
	return "<built-in method capitalize of str object>"
}
func (*stringCapitalizeMethod) isValue() {}

func executeStringCapitalizeCall(
	caller *frame,
	instruction int,
	base int,
	method *stringCapitalizeMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.capitalize() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.capitalize() takes no arguments ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}

	capitalized := capitalizeString(method.value.value)
	discardCallSegment(caller, base)
	if capitalized == method.value.value {
		return pushOutcome(caller, instruction, method.value)
	}
	return pushOutcome(caller, instruction, &stringValue{value: capitalized})
}

func capitalizeString(value string) string {
	if value == "" {
		return value
	}
	_, size, _ := decodeStringRune(value)
	first := value[:size]
	if !utf8.ValidString(first) {
		return first + lowerString(value[size:])
	}
	lowered := lowerString(value)
	loweredFirst := lowerString(first)
	titledFirst := cases.Title(language.Und).String(first)
	return titledFirst + lowered[len(loweredFirst):]
}
