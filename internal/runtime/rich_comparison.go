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
}

// executeUserEquality orders the available operand methods, giving a strict
// right subclass priority before starting the resumable comparison chain.
func executeUserEquality(
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
	return continueComparisonCall(frame, call)
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

// continueComparisonCall invokes the next candidate or applies identity after
// every available method has returned NotImplemented.
func continueComparisonCall(
	frame *frame,
	call *comparisonCall,
) (instructionOutcome, error) {
	if call.next >= len(call.candidates) {
		equal := call.left == call.right
		if call.operand == bytecode.CompareNotEqual {
			equal = !equal
		}
		result := falseSingleton
		if equal {
			result = trueSingleton
		}
		return pushOutcome(frame, call.instruction, result)
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
	return pushOutcome(frame, call.instruction, result)
}
