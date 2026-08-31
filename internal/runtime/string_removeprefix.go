package runtime

import (
	"strconv"
	"strings"
)

type stringRemovePrefixMethod struct {
	value *stringValue
}

func (*stringRemovePrefixMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringRemovePrefixMethod) Repr() string {
	return "<built-in method removeprefix of str object>"
}
func (*stringRemovePrefixMethod) isValue() {}

// executeStringRemovePrefixCall validates one positional string and returns
// the receiver unchanged unless a nonempty prefix matches exactly.
func executeStringRemovePrefixCall(
	caller *frame,
	instruction int,
	base int,
	method *stringRemovePrefixMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.removeprefix() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"str.removeprefix() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	prefix, ok := arguments[0].(*stringValue)
	if !ok {
		typeName := arguments[0].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"removeprefix() argument must be str, not "+typeName,
		)), nil
	}

	discardCallSegment(caller, base)
	if prefix.value == "" || !strings.HasPrefix(method.value.value, prefix.value) {
		return pushOutcome(caller, instruction, method.value)
	}
	return pushOutcome(caller, instruction, &stringValue{
		value: method.value.value[len(prefix.value):],
	})
}
