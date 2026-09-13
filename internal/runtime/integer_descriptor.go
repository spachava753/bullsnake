package runtime

import (
	"math/big"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// addIntegerDescriptors publishes actual native numeric slots. Bool keeps its
// own representation; unsupported bool bitwise overrides are not inherited here.
func addIntegerDescriptors(class *nativeTypeValue, namespace *dictValue) {
	if class == boolNativeType {
		namespace.set(&stringValue{value: "__repr__"}, &nativeDescriptorValue{class: class, name: "__repr__", call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			if exception := checkNativeArguments("__repr__", arguments, keywords, 0, 0); exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(caller, instruction, &stringValue{value: self.Repr()})
		}})
		return
	}
	if class != intNativeType {
		return
	}
	for _, name := range []string{"__repr__", "__format__", "__int__", "__index__", "__bool__", "__pos__", "__neg__", "__invert__", "__abs__", "__getnewargs__", "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__"} {
		kind := nativeWrapperDescriptor
		if name == "__format__" || name == "__getnewargs__" {
			kind = nativeMethodDescriptor
		}
		namespace.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, name: name, kind: kind, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			return executeIntegerSlot(caller, instruction, self, name, arguments, keywords)
		}})
	}
	for _, operand := range []uint32{bytecode.BinaryAdd, bytecode.BinarySubtract, bytecode.BinaryMultiply, bytecode.BinaryDivide, bytecode.BinaryFloorDivide, bytecode.BinaryModulo, bytecode.BinaryPower, bytecode.BinaryLeftShift, bytecode.BinaryRightShift, bytecode.BinaryAnd, bytecode.BinaryOr, bytecode.BinaryXor} {
		normal, reflected, _ := binaryMethodNames(operand)
		for _, name := range []string{normal, reflected} {
			namespace.set(&stringValue{value: name}, &nativeDescriptorValue{class: class, name: name, call: func(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
				if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
					return raiseOutcome(exception), nil
				}
				left, _ := integerOperand(self)
				right, ok := integerOperand(arguments[0])
				if !ok {
					return pushOutcome(caller, instruction, notImplementedSingleton)
				}
				if name == reflected {
					left, right = right, left
				}
				return executeBinaryValues(caller, instruction, operand, false, &intValue{value: left}, &intValue{value: right})
			}})
		}
	}
}

// executeIntegerSlot shares exact numeric storage across unary, comparison,
// conversion and formatting descriptors without redispatching Python overrides.
func executeIntegerSlot(caller *frame, instruction int, self Value, name string, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	arity := 0
	switch name {
	case "__format__", "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__":
		arity = 1
	}
	if exception := checkNativeArguments(name, arguments, keywords, arity, arity); exception != nil {
		return raiseOutcome(exception), nil
	}
	number, _ := integerOperand(self)
	value := &intValue{value: number}
	if exact, ok := self.(*intValue); ok {
		value = exact
	}
	var result Value = value
	switch name {
	case "__repr__":
		result = &stringValue{value: value.Repr()}
	case "__format__":
		return executeIntegerFormat(caller, instruction, self, &number, arguments[0])
	case "__bool__":
		result = booleanValue(number.Sign() != 0)
	case "__neg__":
		result = integerUnary(&number, bytecode.UnaryNegative)
	case "__invert__":
		result = integerUnary(&number, bytecode.UnaryInvert)
	case "__abs__":
		if number.Sign() < 0 {
			result = integerUnary(&number, bytecode.UnaryNegative)
		}
	case "__getnewargs__":
		result = &tupleValue{elements: []Value{value}}
	case "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__":
		right, ok := integerOperand(arguments[0])
		if !ok {
			return pushOutcome(caller, instruction, notImplementedSingleton)
		}
		comparison := number.Cmp(&right)
		result = booleanValue(name == "__eq__" && comparison == 0 || name == "__ne__" && comparison != 0 || name == "__lt__" && comparison < 0 || name == "__le__" && comparison <= 0 || name == "__gt__" && comparison > 0 || name == "__ge__" && comparison >= 0)
	}
	return pushOutcome(caller, instruction, result)
}

// executeIntegerFormat validates the specification and shares the existing
// integer formatter. Empty formatting uses real str conversion, including bool.
func executeIntegerFormat(caller *frame, instruction int, self Value, number *big.Int, specification Value) (instructionOutcome, error) {
	spec, ok := specification.(*stringValue)
	if !ok {
		return raiseOutcome(newException("TypeError", "__format__() argument must be str, not "+specification.TypeName())), nil
	}
	if spec.value == "" {
		return executeString(caller, instruction, self)
	}
	text, exception := formatIntegerValue(number, spec.value, self.TypeName())
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, &stringValue{value: text})
}
