package runtime

const tupleHashPrime5 uint64 = 2870177450012600261

func mixTupleHash(accumulator uint64, hash int64) uint64 {
	accumulator += uint64(hash) * 14029467366897019727
	accumulator = (accumulator << 31) | (accumulator >> 33)
	return accumulator * 11400714785074694791
}

func finishTupleHash(accumulator uint64, length int) int64 {
	accumulator += uint64(length) ^ (tupleHashPrime5 ^ 3527539)
	if accumulator == ^uint64(0) {
		return 1546275796
	}
	return int64(accumulator)
}

type hashCollectionState struct {
	tuple       *tupleValue
	elements    []Value
	index       int
	accumulator uint64
}

type hashCollectionCall struct {
	instruction int
	stack       []hashCollectionState
}

func newHashCollectionState(value Value) (hashCollectionState, bool) {
	switch value := value.(type) {
	case *tupleValue:
		return hashCollectionState{tuple: value, elements: value.elements, accumulator: tupleHashPrime5}, true
	case *frozenSetValue:
		return hashCollectionState{elements: value.entries, accumulator: uint64(len(value.entries)+1) * 1927868237}, true
	}
	return hashCollectionState{}, false
}

// advance keeps native tuple/frozenset nesting on an explicit typed worklist.
// User hashes run through VM continuations, and successful tuple hashes cache.
func (call *hashCollectionCall) advance(caller *frame) (instructionOutcome, error) {
	for len(call.stack) != 0 {
		last := len(call.stack) - 1
		state := &call.stack[last]
		if state.index == len(state.elements) || (state.tuple != nil && state.tuple.hash != nil) {
			hash := state.finish()
			if state.tuple != nil {
				state.tuple.hash = hash
			}
			call.stack[last] = hashCollectionState{}
			call.stack = call.stack[:last]
			if last == 0 {
				return pushOutcome(caller, call.instruction, hash)
			}
			call.stack[last-1].add(hash.value.Int64())
			continue
		}
		element := state.elements[state.index]
		state.index++
		if child, collection := newHashCollectionState(element); collection {
			call.stack = append(call.stack, child)
			continue
		}
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := executeBuiltinHash(caller, call.instruction, len(caller.stack), []Value{element}, nil)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			call.stack[len(call.stack)-1].add(result.(*intValue).value.Int64())
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
	return instructionOutcome{}, caller.failure(call.instruction, "empty collection hash worklist")
}

func (state *hashCollectionState) add(hash int64) {
	if state.tuple != nil {
		state.accumulator = mixTupleHash(state.accumulator, hash)
	} else {
		bits := uint64(hash)
		state.accumulator ^= (bits ^ (bits << 16) ^ 89869747) * 3644798167
	}
}

func (state *hashCollectionState) finish() *intValue {
	if state.tuple != nil {
		if state.tuple.hash != nil {
			return state.tuple.hash
		}
		return hashIntegerValue(finishTupleHash(state.accumulator, len(state.elements)))
	}
	result := state.accumulator
	result ^= (result >> 11) ^ (result >> 25)
	result = result*69069 + 907133923
	if result == ^uint64(0) {
		result = 590923713
	}
	return hashIntegerValue(int64(result))
}
