package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

// rightOperandSubclass orders reflected methods for both user/user and
// native/user operand pairs without calling Python subclass-check hooks.
func rightOperandSubclass(left, right Value) bool {
	rightInstance, ok := right.(*instanceValue)
	if !ok {
		return false
	}
	if leftInstance, ok := left.(*instanceValue); ok {
		return rightInstance.class != leftInstance.class && rightInstance.class.isSubclassOf(leftInstance.class)
	}
	class, _ := typeOf(left)
	if native, ok := class.(*nativeTypeValue); ok {
		return rightInstance.class.isSubclassOfNative(native)
	}
	return false
}

// lookupOperandSpecial adds real native scalar operations to the same candidate
// sequence as Python methods. Internal adapters are not published descriptors.
func lookupOperandSpecial(receiver Value, name string) (Value, bool) {
	if instance, ok := receiver.(*instanceValue); ok {
		return lookupInstanceSpecial(instance, name)
	}
	if _, text := receiver.(*stringValue); text {
		switch name {
		case "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__", "__mod__", "__rmod__":
			return &builtinFunctionValue{name: name, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
				argument := arguments[0]
				discardCallSegment(caller, base)
				if name == "__mod__" || name == "__rmod__" {
					method, _ := caller.runtime.nativeClassAttribute(stringNativeType, name)
					return executeFunctionCall(caller, instruction, len(caller.stack), method, []Value{receiver, argument}, keywords)
				}
				return executeStringSlot(caller, instruction, receiver, name, []Value{argument}, keywords)
			}}, true
		}
		return nil, false
	}
	switch receiver.(type) {
	case *intValue, *boolValue, *floatValue:
	default:
		return nil, false
	}
	switch name {
	case "__eq__", "__ne__", "__lt__", "__le__", "__gt__", "__ge__":
		return &builtinFunctionValue{name: name, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			argument := arguments[0]
			discardCallSegment(caller, base)
			if _, floating := receiver.(*floatValue); !floating {
				return executeIntegerSlot(caller, instruction, receiver, name, []Value{argument}, keywords)
			}
			return pushOutcome(caller, instruction, nativeFloatComparison(receiver, argument, name))
		}}, true
	}
	for _, operand := range []uint32{bytecode.BinaryAdd, bytecode.BinarySubtract, bytecode.BinaryMultiply, bytecode.BinaryDivide, bytecode.BinaryFloorDivide, bytecode.BinaryModulo, bytecode.BinaryPower, bytecode.BinaryLeftShift, bytecode.BinaryRightShift, bytecode.BinaryAnd, bytecode.BinaryOr, bytecode.BinaryXor} {
		normal, reflected, _ := binaryMethodNames(operand)
		if name != normal && name != reflected {
			continue
		}
		return &builtinFunctionValue{name: name, frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			argument := arguments[0]
			discardCallSegment(caller, base)
			if _, floating := receiver.(*floatValue); !floating {
				return executeIntegerBinarySlot(caller, instruction, receiver, name, operand, name == reflected, []Value{argument}, keywords)
			}
			if integer, ok := integerOperand(argument); ok {
				argument = &intValue{value: integer}
			}
			left, right := receiver, argument
			if name == reflected {
				left, right = right, left
			}
			result, exception, supported := floatBinary(left, right, operand)
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if !supported {
				result = notImplementedSingleton
			}
			return pushOutcome(caller, instruction, result)
		}}, true
	}
	return nil, false
}

// nativeFloatComparison compares supported native numeric payloads directly,
// leaving unrelated operands to their reflected comparison methods.
func nativeFloatComparison(left, right Value, name string) Value {
	if name == "__eq__" || name == "__ne__" {
		_, integer := integerOperand(right)
		if !integer {
			switch right.(type) {
			case *floatValue, *complexValue:
			default:
				return notImplementedSingleton
			}
		}
		return booleanValue(valuesEqual(left, right) == (name == "__eq__"))
	}
	comparison, ordered, supported := orderedValues(left, right)
	if !supported {
		return notImplementedSingleton
	}
	return booleanValue(ordered && (name == "__lt__" && comparison < 0 || name == "__le__" && comparison <= 0 || name == "__gt__" && comparison > 0 || name == "__ge__" && comparison >= 0))
}
