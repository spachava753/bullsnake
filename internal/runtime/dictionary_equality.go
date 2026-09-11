package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

type dictionaryEqualityCall struct {
	instruction int
	left, right *dictValue
	index       int
	equal       bool
	invert      bool
	parent      *dictionaryEqualityCall
}

// executeDictionaryEquality compares current entries by key, detects recursive
// dictionary pairs, and retains a parent chain only for nested value comparison.
func executeDictionaryEquality(caller *frame, instruction int, left, right *dictValue, invert bool, parent *dictionaryEqualityCall) (instructionOutcome, error) {
	call := &dictionaryEqualityCall{instruction: instruction, left: left, right: right, equal: len(left.entries) == len(right.entries), invert: invert, parent: parent}
	if left == right || !call.equal {
		return call.finish(caller)
	}
	depth := 0
	for active := parent; active != nil; active = active.parent {
		depth++
		if depth >= 1000 || (active.left == left && active.right == right) {
			return raiseOutcome(newException("RecursionError", "maximum recursion depth exceeded in comparison")), nil
		}
	}
	return call.advance(caller)
}

// advance consumes immediate comparisons in a loop and resumes after Python
// equality or truth callbacks without growing Go stack with dictionary size.
func (call *dictionaryEqualityCall) advance(caller *frame) (instructionOutcome, error) {
	for call.equal && call.index < len(call.left.entries) {
		entry := call.left.entries[call.index]
		call.index++
		right, found, exception := call.right.get(entry.key)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if !found {
			call.equal = false
			break
		}
		if entry.value == right {
			continue
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
				return call.compareValue(caller, entry.value, right)
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
	return call.finish(caller)
}

// compareValue prefers user comparison dispatch, then recursively compares
// dictionary values; other native values retain their current equality rules.
func (call *dictionaryEqualityCall) compareValue(caller *frame, left, right Value) (instructionOutcome, error) {
	_, leftUser := left.(*instanceValue)
	_, rightUser := right.(*instanceValue)
	if leftUser || rightUser {
		return continueComparisonCall(caller, newEqualityCall(call.instruction, bytecode.CompareEqual, left, right))
	}
	leftDict, leftMapping := left.(*dictValue)
	rightDict, rightMapping := right.(*dictValue)
	if leftMapping && rightMapping {
		return executeDictionaryEquality(caller, call.instruction, leftDict, rightDict, false, call)
	}
	result := falseSingleton
	if valuesEqual(left, right) {
		result = trueSingleton
	}
	return pushOutcome(caller, call.instruction, result)
}

func (call *dictionaryEqualityCall) finish(caller *frame) (instructionOutcome, error) {
	result := falseSingleton
	if call.equal != call.invert {
		result = trueSingleton
	}
	return pushOutcome(caller, call.instruction, result)
}
