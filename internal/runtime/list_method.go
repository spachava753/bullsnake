package runtime

import (
	"strconv"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type listAppendMethod struct {
	list *listValue
}

type listPopMethod struct {
	list *listValue
}

type listExtendMethod struct {
	list *listValue
}

type listRemoveMethod struct {
	list *listValue
}

type listSortMethod struct {
	list *listValue
}

type listRemoveCall struct {
	instruction int
	list        *listValue
	needle      Value
	index       int
}

func (*listAppendMethod) TypeName() string { return "builtin_function_or_method" }
func (*listAppendMethod) Repr() string {
	return "<built-in method append of list object>"
}
func (*listAppendMethod) isValue() {}

func (*listPopMethod) TypeName() string { return "builtin_function_or_method" }
func (*listPopMethod) Repr() string {
	return "<built-in method pop of list object>"
}
func (*listPopMethod) isValue() {}

func (*listExtendMethod) TypeName() string { return "builtin_function_or_method" }
func (*listExtendMethod) Repr() string {
	return "<built-in method extend of list object>"
}
func (*listExtendMethod) isValue() {}

func (*listRemoveMethod) TypeName() string { return "builtin_function_or_method" }
func (*listRemoveMethod) Repr() string {
	return "<built-in method remove of list object>"
}
func (*listRemoveMethod) isValue() {}

func (*listSortMethod) TypeName() string { return "builtin_function_or_method" }
func (*listSortMethod) Repr() string {
	return "<built-in method sort of list object>"
}
func (*listSortMethod) isValue() {}

// executeListAttributeLoad binds the implemented native list methods to their
// receiver and reports ordinary missing attributes.
func executeListAttributeLoad(
	frame *frame,
	instruction int,
	list *listValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "append":
		return pushOutcome(frame, instruction, &listAppendMethod{list: list})
	case "pop":
		return pushOutcome(frame, instruction, &listPopMethod{list: list})
	case "extend":
		return pushOutcome(frame, instruction, &listExtendMethod{list: list})
	case "remove":
		return pushOutcome(frame, instruction, &listRemoveMethod{list: list})
	case "sort":
		return pushOutcome(frame, instruction, &listSortMethod{list: list})
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'list' object has no attribute '"+name+"'",
		)), nil
	}
}

func executeListAppendCall(
	caller *frame,
	instruction int,
	base int,
	method *listAppendMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.append() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.append() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	method.list.elements = append(method.list.elements, arguments[0])
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, None)
}

// executeListExtendCall starts iterator-driven in-place extension and snapshots
// direct self-extension before the target begins to grow.
func executeListExtendCall(
	caller *frame,
	instruction int,
	base int,
	method *listExtendMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.extend() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.extend() takes exactly one argument ("+count+" given)",
		)), nil
	}

	iterable := arguments[0]
	if source, sameType := iterable.(*listValue); sameType && source == method.list {
		elements := make([]Value, len(source.elements))
		copy(elements, source.elements)
		iterable = &tupleValue{elements: elements}
	}
	call := &collectionConstructorCall{
		instruction: instruction,
		kind:        collectionListExtend,
		iterable:    iterable,
		list:        method.list,
	}
	discardCallSegment(caller, base)
	return startCollectionConstructor(caller, call)
}

// executeListPopCall removes one selected element after applying Python's
// positional-only integer index rules.
func executeListPopCall(
	caller *frame,
	instruction int,
	base int,
	method *listPopMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.pop() takes no keyword arguments",
		)), nil
	}
	if len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"pop expected at most 1 argument, got "+strconv.Itoa(len(arguments)),
		)), nil
	}

	index := -1
	if len(arguments) == 1 {
		integer, ok := integerOperand(arguments[0])
		if !ok {
			message := "'" + arguments[0].TypeName() +
				"' object cannot be interpreted as an integer"
			discardCallSegment(caller, base)
			return raiseOutcome(newException("TypeError", message)), nil
		}
		if !integer.IsInt64() || int64(int(integer.Int64())) != integer.Int64() {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"OverflowError",
				"Python int too large to convert to C ssize_t",
			)), nil
		}
		index = int(integer.Int64())
	}

	if len(method.list.elements) == 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException("IndexError", "pop from empty list")), nil
	}
	if index < 0 {
		index += len(method.list.elements)
	}
	if index < 0 || index >= len(method.list.elements) {
		discardCallSegment(caller, base)
		return raiseOutcome(newException("IndexError", "pop index out of range")), nil
	}

	value := method.list.elements[index]
	copy(method.list.elements[index:], method.list.elements[index+1:])
	last := len(method.list.elements) - 1
	method.list.elements[last] = nil
	method.list.elements = method.list.elements[:last]
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, value)
}

// executeListRemoveCall validates the bound method arguments and starts the
// resumable identity-or-equality scan.
func executeListRemoveCall(
	caller *frame,
	instruction int,
	base int,
	method *listRemoveMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.remove() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"list.remove() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}

	call := &listRemoveCall{
		instruction: instruction,
		list:        method.list,
		needle:      arguments[0],
	}
	discardCallSegment(caller, base)
	return continueListRemoveCall(caller, call)
}

// executeListSortCall snapshots the current elements and mutates the receiver
// only after the shared resumable sort completes successfully.
func executeListSortCall(
	caller *frame,
	instruction int,
	base int,
	method *listSortMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if len(arguments) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"sort() takes no positional arguments",
		)), nil
	}
	key, reverse, exception := bindSortControls(keywords)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	values := append([]Value(nil), method.list.elements...)
	call := &sortCall{
		instruction:  instruction,
		key:          key,
		reverseValue: reverse,
		target:       method.list,
	}
	discardCallSegment(caller, base)
	return startSortValues(caller, call, values)
}

// continueListRemoveCall scans fixed values inline and suspends only when a
// user equality method must run in another Python frame.
func continueListRemoveCall(
	frame *frame,
	call *listRemoveCall,
) (instructionOutcome, error) {
	for call.index < len(call.list.elements) {
		element := call.list.elements[call.index]
		if element == call.needle {
			return finishListRemoveTruth(frame, call, true)
		}
		_, elementUser := element.(*instanceValue)
		_, needleUser := call.needle.(*instanceValue)
		if elementUser || needleUser {
			comparison := newEqualityCall(
				call.instruction,
				bytecode.CompareEqual,
				element,
				call.needle,
			)
			comparison.listRemoval = call
			return continueComparisonCall(frame, comparison)
		}
		if valuesEqual(element, call.needle) {
			return finishListRemoveTruth(frame, call, true)
		}
		call.index++
	}
	return raiseOutcome(newException(
		"ValueError",
		"list.remove(x): x not in list",
	)), nil
}

// finishListRemoveTruth applies one comparison result and resumes the dynamic
// scan when it did not match.
func finishListRemoveTruth(
	frame *frame,
	call *listRemoveCall,
	matches bool,
) (instructionOutcome, error) {
	if !matches {
		call.index++
		return continueListRemoveCall(frame, call)
	}
	if call.index < len(call.list.elements) {
		copy(call.list.elements[call.index:], call.list.elements[call.index+1:])
		last := len(call.list.elements) - 1
		call.list.elements[last] = nil
		call.list.elements = call.list.elements[:last]
	}
	return pushOutcome(frame, call.instruction, None)
}
