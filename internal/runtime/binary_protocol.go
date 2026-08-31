package runtime

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type binaryCandidate struct {
	method   Value
	argument Value
}

type binaryCall struct {
	instruction int
	operand     uint32
	inPlace     bool
	left        Value
	right       Value
	candidates  []binaryCandidate
	next        int
}

// executeUserBinary builds the in-place, normal, and reflected method order,
// giving a strict right subclass priority within the ordinary pair.
func executeUserBinary(
	frame *frame,
	instruction int,
	operand uint32,
	inPlace bool,
	left Value,
	right Value,
) (instructionOutcome, error) {
	normal, reflected, inplace := binaryMethodNames(operand)
	call := &binaryCall{
		instruction: instruction,
		operand:     operand,
		inPlace:     inPlace,
		left:        left,
		right:       right,
	}
	leftInstance, leftUser := left.(*instanceValue)
	rightInstance, rightUser := right.(*instanceValue)
	if inPlace && leftUser {
		call.appendCandidate(leftInstance, right, inplace)
	}
	if leftUser && rightUser && rightInstance.class != leftInstance.class &&
		rightInstance.class.isSubclassOf(leftInstance.class) {
		call.appendCandidate(rightInstance, left, reflected)
		call.appendCandidate(leftInstance, right, normal)
	} else {
		if leftUser {
			call.appendCandidate(leftInstance, right, normal)
		}
		if rightUser {
			call.appendCandidate(rightInstance, left, reflected)
		}
	}
	return continueBinaryCall(frame, call)
}

func (call *binaryCall) appendCandidate(
	receiver *instanceValue,
	argument Value,
	name string,
) {
	method, found := lookupInstanceSpecial(receiver, name)
	if !found {
		return
	}
	call.candidates = append(call.candidates, binaryCandidate{
		method:   method,
		argument: argument,
	})
}

// continueBinaryCall invokes candidates until one returns a concrete value, or
// raises the ordinary operator error after every candidate declines.
func continueBinaryCall(
	frame *frame,
	call *binaryCall,
) (instructionOutcome, error) {
	if call.next >= len(call.candidates) {
		operator := binaryOperatorSymbol(call.operand)
		if call.inPlace {
			operator += "="
		}
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				fmt.Sprintf(
					"unsupported operand type(s) for %s: '%s' and '%s'",
					operator,
					call.left.TypeName(),
					call.right.TypeName(),
				),
			),
		}, nil
	}

	candidate := call.candidates[call.next]
	call.next++
	outcome, err := executeFunctionCall(
		frame,
		call.instruction,
		len(frame.stack),
		candidate.method,
		[]Value{candidate.argument},
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.binary = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"binary special method returned without a value",
		)
	}
	return finishBinaryCall(frame, call, result)
}

func finishBinaryCall(
	frame *frame,
	call *binaryCall,
	result Value,
) (instructionOutcome, error) {
	if result == notImplementedSingleton {
		return continueBinaryCall(frame, call)
	}
	return pushOutcome(frame, call.instruction, result)
}

// binaryMethodNames maps one bytecode operand to its normal, reflected, and
// in-place Python special methods.
func binaryMethodNames(operand uint32) (normal, reflected, inplace string) {
	switch operand {
	case bytecode.BinaryAdd:
		return "__add__", "__radd__", "__iadd__"
	case bytecode.BinarySubtract:
		return "__sub__", "__rsub__", "__isub__"
	case bytecode.BinaryMultiply:
		return "__mul__", "__rmul__", "__imul__"
	case bytecode.BinaryMatrixMultiply:
		return "__matmul__", "__rmatmul__", "__imatmul__"
	case bytecode.BinaryDivide:
		return "__truediv__", "__rtruediv__", "__itruediv__"
	case bytecode.BinaryFloorDivide:
		return "__floordiv__", "__rfloordiv__", "__ifloordiv__"
	case bytecode.BinaryModulo:
		return "__mod__", "__rmod__", "__imod__"
	case bytecode.BinaryPower:
		return "__pow__", "__rpow__", "__ipow__"
	case bytecode.BinaryLeftShift:
		return "__lshift__", "__rlshift__", "__ilshift__"
	case bytecode.BinaryRightShift:
		return "__rshift__", "__rrshift__", "__irshift__"
	case bytecode.BinaryOr:
		return "__or__", "__ror__", "__ior__"
	case bytecode.BinaryXor:
		return "__xor__", "__rxor__", "__ixor__"
	default:
		return "__and__", "__rand__", "__iand__"
	}
}

// binaryOperatorSymbol returns the source spelling used by normal and in-place
// unsupported-operand errors.
func binaryOperatorSymbol(operand uint32) string {
	switch operand {
	case bytecode.BinarySubtract:
		return "-"
	case bytecode.BinaryMultiply:
		return "*"
	case bytecode.BinaryMatrixMultiply:
		return "@"
	case bytecode.BinaryDivide:
		return "/"
	case bytecode.BinaryFloorDivide:
		return "//"
	case bytecode.BinaryModulo:
		return "%"
	case bytecode.BinaryPower:
		return "**"
	case bytecode.BinaryLeftShift:
		return "<<"
	case bytecode.BinaryRightShift:
		return ">>"
	case bytecode.BinaryOr:
		return "|"
	case bytecode.BinaryXor:
		return "^"
	case bytecode.BinaryAnd:
		return "&"
	default:
		return "+"
	}
}
