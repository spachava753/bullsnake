package runtime

import "strconv"

type setAddMethod struct {
	set *setValue
}

type setDiscardMethod struct {
	set *setValue
}

type setContainsTarget interface {
	Value
	contains(Value) (bool, *Exception)
}

type setContainsMethod struct {
	target setContainsTarget
}

type setContainsDescriptor struct {
	frozen bool
}

type setDifferenceMethod struct {
	target setContainsTarget
}

func (*setAddMethod) TypeName() string { return "builtin_function_or_method" }
func (*setAddMethod) Repr() string {
	return "<built-in method add of set object>"
}
func (*setAddMethod) isValue() {}

func (*setDiscardMethod) TypeName() string { return "builtin_function_or_method" }
func (*setDiscardMethod) Repr() string {
	return "<built-in method discard of set object>"
}
func (*setDiscardMethod) isValue() {}

func (*setContainsMethod) TypeName() string { return "builtin_function_or_method" }
func (method *setContainsMethod) Repr() string {
	return "<built-in method __contains__ of " + method.target.TypeName() + " object>"
}
func (*setContainsMethod) isValue() {}

func (*setContainsDescriptor) TypeName() string { return "method_descriptor" }
func (descriptor *setContainsDescriptor) Repr() string {
	return "<method '__contains__' of '" + descriptor.typeName() + "' objects>"
}
func (*setContainsDescriptor) isValue() {}

func (*setDifferenceMethod) TypeName() string { return "builtin_function_or_method" }
func (method *setDifferenceMethod) Repr() string {
	return "<built-in method difference of " + method.target.TypeName() + " object>"
}
func (*setDifferenceMethod) isValue() {}

func (descriptor *setContainsDescriptor) typeName() string {
	if descriptor.frozen {
		return "frozenset"
	}
	return "set"
}

func executeSetAttributeLoad(
	frame *frame,
	instruction int,
	set *setValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "__contains__":
		return pushOutcome(frame, instruction, &setContainsMethod{target: set})
	case "add":
		return pushOutcome(frame, instruction, &setAddMethod{set: set})
	case "difference":
		return pushOutcome(frame, instruction, &setDifferenceMethod{target: set})
	case "discard":
		return pushOutcome(frame, instruction, &setDiscardMethod{set: set})
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'set' object has no attribute '"+name+"'",
		)), nil
	}
}

func executeFrozenSetAttributeLoad(
	frame *frame,
	instruction int,
	set *frozenSetValue,
	name string,
) (instructionOutcome, error) {
	switch name {
	case "__contains__":
		return pushOutcome(frame, instruction, &setContainsMethod{target: set})
	case "difference":
		return pushOutcome(frame, instruction, &setDifferenceMethod{target: set})
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"'frozenset' object has no attribute '"+name+"'",
		)), nil
	}
}

func executeSetAddCall(
	caller *frame,
	instruction int,
	base int,
	method *setAddMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"set.add() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"set.add() takes exactly one argument ("+count+" given)",
		)), nil
	}
	exception := method.set.add(arguments[0])
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, None)
}

func executeSetDiscardCall(
	caller *frame,
	instruction int,
	base int,
	method *setDiscardMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"set.discard() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"set.discard() takes exactly one argument ("+count+" given)",
		)), nil
	}
	_, exception := method.set.discard(arguments[0])
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, None)
}

// executeSetContainsCall validates the bound-call shape, delegates membership to
// the receiver, and returns the canonical boolean singleton.
func executeSetContainsCall(
	caller *frame,
	instruction int,
	base int,
	method *setContainsMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	name := method.target.TypeName()
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			name+".__contains__() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		count := strconv.Itoa(len(arguments))
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			name+".__contains__() takes exactly one argument ("+count+" given)",
		)), nil
	}
	contained, exception := method.target.contains(arguments[0])
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if contained {
		return pushOutcome(caller, instruction, trueSingleton)
	}
	return pushOutcome(caller, instruction, falseSingleton)
}

func executeSetContainsDescriptorCall(
	caller *frame,
	instruction int,
	base int,
	descriptor *setContainsDescriptor,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	name := descriptor.typeName()
	if len(arguments) == 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"unbound method "+name+".__contains__() needs an argument",
		)), nil
	}
	var target setContainsTarget
	var valid bool
	if descriptor.frozen {
		var frozen *frozenSetValue
		frozen, valid = arguments[0].(*frozenSetValue)
		target = frozen
	} else {
		var mutable *setValue
		mutable, valid = arguments[0].(*setValue)
		target = mutable
	}
	if !valid {
		actual := arguments[0].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"descriptor '__contains__' for '"+name+
				"' objects doesn't apply to a '"+actual+"' object",
		)), nil
	}
	return executeSetContainsCall(
		caller,
		instruction,
		base,
		&setContainsMethod{target: target},
		arguments[1:],
		keywords,
	)
}

// executeSetDifferenceCall copies the receiver, retains every argument source,
// and starts the shared iterator continuation for the first one.
func executeSetDifferenceCall(
	caller *frame,
	instruction int,
	base int,
	method *setDifferenceMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"difference() takes no keyword arguments",
		)), nil
	}
	var receiver []Value
	frozen := false
	switch target := method.target.(type) {
	case *setValue:
		receiver = target.entries
	case *frozenSetValue:
		receiver = target.entries
		frozen = true
	}
	call := &collectionConstructorCall{
		instruction: instruction,
		kind:        collectionSetDifference,
		set: &setValue{
			entries: append([]Value(nil), receiver...),
		},
		frozen: frozen,
	}
	if len(arguments) != 0 {
		sources := append([]Value(nil), arguments...)
		call.iterable = sources[0]
		call.iterables = sources[1:]
	}
	discardCallSegment(caller, base)
	if call.iterable == nil {
		return finishSetDifference(caller, call)
	}
	return startCollectionConstructor(caller, call)
}

// finishSetDifference removes one completed source's items, then advances to
// the next iterable or returns a new result with the receiver's mutability.
func finishSetDifference(
	frame *frame,
	call *collectionConstructorCall,
) (instructionOutcome, error) {
	for index, element := range call.elements {
		if _, exception := call.set.discard(element); exception != nil {
			return raiseOutcome(exception), nil
		}
		call.elements[index] = nil
	}
	call.elements = call.elements[:0]
	if len(call.iterables) != 0 {
		call.iterable = call.iterables[0]
		call.iterables[0] = nil
		call.iterables = call.iterables[1:]
		call.iterator = nil
		return startCollectionConstructor(frame, call)
	}
	if call.frozen {
		return pushOutcome(frame, call.instruction, &frozenSetValue{
			entries: call.set.entries,
		})
	}
	return pushOutcome(frame, call.instruction, call.set)
}
