package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

type comparisonCandidate struct {
	method   Value
	argument Value
	invert   bool
}

type comparisonCall struct {
	instruction int
	operand     uint32
	left        Value
	right       Value
	candidates  []comparisonCandidate
	next        int
	invert      bool
	listRemoval *listRemoveCall
}

// newEqualityCall records the user comparison candidates so bytecode and
// native operations can share the same resumable ordering.
func newEqualityCall(
	instruction int,
	operand uint32,
	left Value,
	right Value,
) *comparisonCall {
	call := &comparisonCall{
		instruction: instruction,
		operand:     operand,
		left:        left,
		right:       right,
	}
	leftInstance, leftUser := left.(*instanceValue)
	rightInstance, rightUser := right.(*instanceValue)
	if leftUser && rightUser && rightInstance.class != leftInstance.class &&
		rightInstance.class.isSubclassOf(leftInstance.class) {
		call.appendEqualityCandidate(rightInstance, left)
		call.appendEqualityCandidate(leftInstance, right)
	} else {
		if leftUser {
			call.appendEqualityCandidate(leftInstance, right)
		}
		if rightUser {
			call.appendEqualityCandidate(rightInstance, left)
		}
	}
	return call
}

// executeUserOrdering maps the source operator to left and reflected method
// names, then applies the same strict-subclass ordering as equality.
func executeUserOrdering(
	frame *frame,
	instruction int,
	operand uint32,
	left Value,
	right Value,
) (instructionOutcome, error) {
	call := &comparisonCall{
		instruction: instruction,
		operand:     operand,
		left:        left,
		right:       right,
	}
	leftName, rightName := orderingMethodNames(operand)
	leftInstance, leftUser := left.(*instanceValue)
	rightInstance, rightUser := right.(*instanceValue)
	if leftUser && rightUser && rightInstance.class != leftInstance.class &&
		rightInstance.class.isSubclassOf(leftInstance.class) {
		call.appendOrderingCandidate(rightInstance, left, rightName)
		call.appendOrderingCandidate(leftInstance, right, leftName)
	} else {
		if leftUser {
			call.appendOrderingCandidate(leftInstance, right, leftName)
		}
		if rightUser {
			call.appendOrderingCandidate(rightInstance, left, rightName)
		}
	}
	return continueComparisonCall(frame, call)
}

func (call *comparisonCall) appendOrderingCandidate(
	receiver *instanceValue,
	argument Value,
	name string,
) {
	method, found := lookupInstanceSpecial(receiver, name)
	if !found {
		return
	}
	call.candidates = append(call.candidates, comparisonCandidate{
		method:   method,
		argument: argument,
	})
}

func orderingMethodNames(operand uint32) (string, string) {
	switch operand {
	case bytecode.CompareLess:
		return "__lt__", "__gt__"
	case bytecode.CompareLessEqual:
		return "__le__", "__ge__"
	case bytecode.CompareGreater:
		return "__gt__", "__lt__"
	default:
		return "__ge__", "__le__"
	}
}

func (call *comparisonCall) appendEqualityCandidate(
	receiver *instanceValue,
	argument Value,
) {
	name := "__eq__"
	invert := false
	method, found := lookupInstanceSpecial(receiver, name)
	if call.operand == bytecode.CompareNotEqual {
		name = "__ne__"
		method, found = lookupInstanceSpecial(receiver, name)
		if !found {
			method, found = lookupInstanceSpecial(receiver, "__eq__")
			invert = found
		}
	}
	if !found {
		return
	}
	call.candidates = append(call.candidates, comparisonCandidate{
		method:   method,
		argument: argument,
		invert:   invert,
	})
}

// continueComparisonCall invokes the next candidate or applies the operator's
// identity or unsupported-ordering fallback after every method declines.
func continueComparisonCall(
	frame *frame,
	call *comparisonCall,
) (instructionOutcome, error) {
	if call.next >= len(call.candidates) {
		return finishDeclinedComparison(frame, call)
	}

	candidate := call.candidates[call.next]
	call.next++
	call.invert = candidate.invert
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
		outcome.frame.comparison = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"comparison special method returned without a value",
		)
	}
	return finishComparisonCall(frame, call, result)
}

// finishDeclinedComparison applies identity for equality and constructs the
// operator-specific TypeError for an ordering declined by both operands.
func finishDeclinedComparison(
	frame *frame,
	call *comparisonCall,
) (instructionOutcome, error) {
	if call.operand == bytecode.CompareEqual || call.operand == bytecode.CompareNotEqual {
		equal := call.left == call.right
		if call.operand == bytecode.CompareNotEqual {
			equal = !equal
		}
		result := falseSingleton
		if equal {
			result = trueSingleton
		}
		return finishComparisonResult(frame, call, result)
	}

	operator := "<"
	switch call.operand {
	case bytecode.CompareLessEqual:
		operator = "<="
	case bytecode.CompareGreater:
		operator = ">"
	case bytecode.CompareGreaterEqual:
		operator = ">="
	}
	return instructionOutcome{
		kind: raised,
		exception: newException(
			"TypeError",
			"'"+operator+"' not supported between instances of '"+
				call.left.TypeName()+"' and '"+call.right.TypeName()+"'",
		),
	}, nil
}

func finishComparisonCall(
	frame *frame,
	call *comparisonCall,
	result Value,
) (instructionOutcome, error) {
	if result == notImplementedSingleton {
		return continueComparisonCall(frame, call)
	}
	if call.invert {
		return executeTruthOperation(
			frame,
			call.instruction,
			bytecode.Instruction{
				Opcode:  bytecode.CompareOp,
				Operand: bytecode.CompareNotEqual,
			},
			result,
		)
	}
	return finishComparisonResult(frame, call, result)
}

// finishComparisonResult lets native operations request boolean conversion of
// a rich comparison without changing ordinary comparison expression results.
func finishComparisonResult(
	frame *frame,
	call *comparisonCall,
	result Value,
) (instructionOutcome, error) {
	if call.listRemoval != nil {
		truth := &truthCall{
			instruction: call.instruction,
			original:    result,
			listRemoval: call.listRemoval,
		}
		return executeTruthWithCall(frame, result, truth)
	}
	return pushOutcome(frame, call.instruction, result)
}
