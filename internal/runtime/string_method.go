package runtime

import (
	"strconv"
	"strings"
)

type stringJoinMethod struct {
	separator *stringValue
}

func (*stringJoinMethod) TypeName() string { return "builtin_function_or_method" }
func (*stringJoinMethod) Repr() string {
	return "<built-in method join of str object>"
}
func (*stringJoinMethod) isValue() {}

func executeStringAttributeLoad(
	frame *frame,
	instruction int,
	value *stringValue,
	name string,
) (instructionOutcome, error) {
	if name == "join" {
		return pushOutcome(frame, instruction, &stringJoinMethod{separator: value})
	}
	return raiseOutcome(newException(
		"AttributeError",
		"'str' object has no attribute '"+name+"'",
	)), nil
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
