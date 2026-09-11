package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

type equalityPath struct {
	left, right Value
	parent      *equalityPath
	depth       int
}

// nextEqualityPath rejects repeated container pairs and bounds native nesting
// before retaining a typed parent link for the next comparison.
func nextEqualityPath(left, right Value, parent *equalityPath) (*equalityPath, *Exception) {
	depth := 1
	if parent != nil {
		depth += parent.depth
	}
	for current := parent; current != nil; current = current.parent {
		if depth > 1000 || (current.left == left && current.right == right) {
			return nil, newException("RecursionError", "maximum recursion depth exceeded in comparison")
		}
	}
	return &equalityPath{left: left, right: right, parent: parent, depth: depth}, nil
}

// executeEqualityValue provides identity preference inside containers before
// user callbacks, native recursive comparison, or fixed scalar equality.
func executeEqualityValue(caller *frame, instruction int, left, right Value, parent *equalityPath) (instructionOutcome, error) {
	if left == right {
		return pushOutcome(caller, instruction, trueSingleton)
	}
	_, leftUser := left.(*instanceValue)
	_, rightUser := right.(*instanceValue)
	if leftUser || rightUser {
		return continueComparisonCall(caller, newEqualityCall(instruction, bytecode.CompareEqual, left, right))
	}
	leftDict, leftMapping := left.(*dictValue)
	rightDict, rightMapping := right.(*dictValue)
	if leftMapping && rightMapping {
		return executeDictionaryEquality(caller, instruction, leftDict, rightDict, false, parent)
	}
	if _, _, sequences := nativeSequencePair(left, right); sequences {
		return executeSequenceEquality(caller, instruction, left, right, false, parent)
	}
	return pushOutcome(caller, instruction, booleanValue(valuesEqual(left, right)))
}

func nativeSequencePair(left, right Value) ([]Value, []Value, bool) {
	switch left := left.(type) {
	case *tupleValue:
		if right, ok := right.(*tupleValue); ok {
			return left.elements, right.elements, true
		}
	case *listValue:
		if right, ok := right.(*listValue); ok {
			return left.elements, right.elements, true
		}
	}
	return nil, nil, false
}

type sequenceEqualityCall struct {
	instruction   int
	left, right   Value
	index         int
	equal, invert bool
	path          *equalityPath
}

func executeSequenceEquality(caller *frame, instruction int, left, right Value, invert bool, parent *equalityPath) (instructionOutcome, error) {
	first, second, _ := nativeSequencePair(left, right)
	_, list := left.(*listValue)
	if left == right || (list && len(first) != len(second)) {
		return pushOutcome(caller, instruction, booleanValue((left == right) != invert))
	}
	path, exception := nextEqualityPath(left, right, parent)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	call := &sequenceEqualityCall{instruction: instruction, left: left, right: right, equal: true, invert: invert, path: path}
	return call.advance(caller)
}

// advance rereads live list storage between callbacks, short-circuits unequal
// elements, and uses a trampoline for immediately completed comparisons.
func (call *sequenceEqualityCall) advance(caller *frame) (instructionOutcome, error) {
	for call.equal {
		left, right, _ := nativeSequencePair(call.left, call.right)
		if call.index >= len(left) || call.index >= len(right) {
			call.equal = len(left) == len(right)
			break
		}
		first, second := left[call.index], right[call.index]
		call.index++
		if first == second {
			continue
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
				return executeEqualityValue(caller, call.instruction, first, second, call.path)
			}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				return executeBuiltinBool(current, call.instruction, len(current.stack), []Value{result}, nil)
			})
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			call.equal = result == trueSingleton
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	return pushOutcome(caller, call.instruction, booleanValue(call.equal != call.invert))
}
