package runtime

import "strings"

// unionValue retains normalized concrete classes in first-occurrence order.
// Aliases and parameterized typing values need their own substitution machinery.
type unionValue struct {
	args       *tupleValue
	parameters tupleValue
}

func (*unionValue) TypeName() string { return "typing.Union" }
func (*unionValue) isValue()         {}
func (value *unionValue) Repr() string {
	parts := make([]string, len(value.args.elements))
	for index, item := range value.args.elements {
		if item == noneNativeType {
			parts[index] = "None"
		} else {
			parts[index] = genericAliasArgumentRepr(item)
		}
	}
	return strings.Join(parts, " | ")
}

func isUnionValue(value Value) bool {
	_, ok := value.(*unionValue)
	return ok
}

func isConcreteUnionOperand(value Value) bool {
	_, union := value.(*unionValue)
	return union || value == None || isClassValue(value)
}

// concreteUnion flattens already-normalized unions without recursing and keeps
// class identity equality, the currently supported class key contract.
func concreteUnion(left, right Value) Value {
	if !isConcreteUnionOperand(left) || !isConcreteUnionOperand(right) {
		return notImplementedSingleton
	}
	var args []Value
	seen := make(map[Value]bool)
	for _, operand := range []Value{left, right} {
		items := []Value{operand}
		if union, ok := operand.(*unionValue); ok {
			items = union.args.elements
		}
		for _, item := range items {
			if item == None {
				item = noneNativeType
			}
			if !seen[item] {
				seen[item] = true
				args = append(args, item)
			}
		}
	}
	if len(args) == 1 {
		return args[0]
	}
	return &unionValue{args: &tupleValue{elements: args}}
}

// executeUnionAttribute exposes immutable metadata and binds implemented union
// operators; parameter metadata is empty because only concrete classes enter.
func executeUnionAttribute(caller *frame, instruction int, value *unionValue, name string) (instructionOutcome, error) {
	switch name {
	case "__args__":
		return pushOutcome(caller, instruction, value.args)
	case "__parameters__":
		return pushOutcome(caller, instruction, &value.parameters)
	case "__origin__":
		return pushOutcome(caller, instruction, nativeTypesByRuntimeName["typing.Union"])
	case "__name__", "__qualname__":
		return pushOutcome(caller, instruction, &stringValue{value: "Union"})
	case "__module__":
		return pushOutcome(caller, instruction, &stringValue{value: "typing"})
	}
	if method, found := caller.runtime.unionBinaryMethod(value, name); found {
		return pushOutcome(caller, instruction, method)
	}
	return raiseOutcome(newException("AttributeError", "'typing.Union' object has no attribute '"+name+"'")), nil
}

// concreteUnionKeyError keeps unsupported metaclass key callbacks out of the
// identity-only union representation instead of silently ignoring those slots.
func concreteUnionKeyError(left, right Value) *Exception {
	if !isConcreteUnionOperand(left) || !isConcreteUnionOperand(right) {
		return nil
	}
	for _, value := range []Value{left, right} {
		items := []Value{value}
		if union, ok := value.(*unionValue); ok {
			items = union.args.elements
		}
		for _, item := range items {
			if class, ok := item.(*typeValue); ok && class.metaclass != nil {
				for _, name := range []string{"__eq__", "__hash__"} {
					if _, found := class.metaclass.lookup(name); found {
						return newException("NotImplementedError", "unions with metaclass equality or hashing overrides are not supported")
					}
				}
			}
		}
	}
	return nil
}

// addUnionDescriptors publishes executable normal and reflected union slots on
// type and Union, sharing operand normalization and unsupported-key checks.
func addUnionDescriptors(class *nativeTypeValue, dictionary *dictValue) {
	if class != typeNativeType && class != nativeTypesByRuntimeName["typing.Union"] {
		return
	}
	for _, name := range []string{"__or__", "__ror__"} {
		dictionary.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
				return raiseOutcome(exception), nil
			}
			left, right := self, arguments[0]
			if name == "__ror__" {
				left, right = right, left
			}
			if exception := concreteUnionKeyError(left, right); exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(caller, instruction, concreteUnion(left, right))
		}})
	}
}

// unionBinaryMethod resolves class operators on the metaclass, never on the
// operand class itself. Native fallbacks use the same exposed slot descriptors.
func (runtime *Runtime) unionBinaryMethod(receiver Value, name string) (Value, bool) {
	if instance, ok := receiver.(*instanceValue); ok {
		return lookupInstanceSpecial(instance, name)
	}
	if name != "__or__" && name != "__ror__" {
		return nil, false
	}
	if class, ok := receiver.(*typeValue); ok && class.metaclass != nil {
		if method, found := class.metaclass.lookup(name); found {
			if bound, descriptor := bindMethodDescriptor(method, class.metaclass); descriptor {
				return bound, true
			}
			return &boundMethodValue{callable: method, self: class}, true
		}
	}
	var owner *nativeTypeValue
	if isClassValue(receiver) {
		owner = typeNativeType
	} else if _, ok := receiver.(*unionValue); ok {
		owner = nativeTypesByRuntimeName["typing.Union"]
	} else {
		return nil, false
	}
	method, _, _ := runtime.nativeNamespace(owner).get(&stringValue{value: name})
	return &boundNativeDescriptorValue{descriptor: method.(*nativeDescriptorValue), self: receiver}, true
}

// unionReflectedOverride compares the underlying reflected slots rather than
// the freshly bound method objects, so inherited slots do not steal priority.
func (runtime *Runtime) unionReflectedOverride(left, right Value) bool {
	underlying := func(receiver Value) Value {
		method, _ := runtime.unionBinaryMethod(receiver, "__ror__")
		switch method := method.(type) {
		case *boundMethodValue:
			return method.callable
		case *boundNativeDescriptorValue:
			return method.descriptor
		default:
			return method
		}
	}
	return underlying(left) != underlying(right)
}

// executeUnionBinary includes metaclass slots in the normal binary continuation.
// A strict right metaclass subclass gets the reflected attempt before the left.
func executeUnionBinary(caller *frame, instruction int, operand uint32, inPlace bool, left, right Value) (instructionOutcome, error) {
	call := &binaryCall{instruction: instruction, operand: operand, inPlace: inPlace, left: left, right: right}
	appendCandidate := func(receiver, argument Value, name string) {
		if method, found := caller.runtime.unionBinaryMethod(receiver, name); found {
			call.candidates = append(call.candidates, binaryCandidate{method: method, argument: argument})
		}
	}
	if inPlace {
		appendCandidate(left, right, "__ior__")
	}
	leftType, _ := typeOf(left)
	rightType, _ := typeOf(right)
	priority, _ := subclassMatchesClass(rightType, leftType)
	if leftType != rightType && priority && caller.runtime.unionReflectedOverride(left, right) {
		appendCandidate(right, left, "__ror__")
		appendCandidate(left, right, "__or__")
	} else {
		appendCandidate(left, right, "__or__")
		if leftType != rightType {
			appendCandidate(right, left, "__ror__")
		}
	}
	return continueBinaryCall(caller, call)
}
