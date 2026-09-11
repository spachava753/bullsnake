package runtime

import (
	"math"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type sumCall struct {
	instruction int
	iterator    Value
	sentinel    Value
	total       Value
	item        Value
	adding      bool
	done        bool
	mode        uint8 // 0: bounded native integer prefix; 1: float; 2: general addition.
	high, low   float64
}

// executeBuiltinSum resolves iteration before validating the start value, then
// folds through ordinary addition rather than mutating the supplied accumulator.
func executeBuiltinSum(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("sum", arguments, nil, 1, 2); exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	source := arguments[0]
	start := Value(integerFromInt64(0))
	if len(arguments) == 2 {
		start = arguments[1]
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				discardCallSegment(caller, base)
				return raiseOutcome(newException("TypeError", "keywords must be strings")), nil
			}
			if name.value != "start" {
				discardCallSegment(caller, base)
				return raiseOutcome(newException("TypeError", "sum() got an unexpected keyword argument '"+name.value+"'")), nil
			}
			if len(arguments) == 2 {
				discardCallSegment(caller, base)
				return raiseOutcome(newException("TypeError", "sum() got multiple values for argument 'start'")), nil
			}
			start = entry.value
		}
	}
	discardCallSegment(caller, base)
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIteratorLookup(caller, instruction, source)
	}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		switch start.(type) {
		case *stringValue:
			return raiseOutcome(newException("TypeError", "sum() can't sum strings [use ''.join(seq) instead]")), nil
		case *bytesValue, *bytearrayValue:
			return raiseOutcome(newException("TypeError", "sum() can't sum "+start.TypeName()+" [use b''.join(seq) instead]")), nil
		}
		call := &sumCall{instruction: instruction, iterator: iterator, sentinel: &dictValue{}, total: start, mode: 2}
		if integer, ok := start.(*intValue); ok && integer.value.IsInt64() {
			call.mode = 0
		} else if number, ok := start.(*floatValue); ok {
			call.mode, call.high = 1, number.value
		}
		return call.advance(current)
	})
}

// advance alternates iterator pulls and additions, using a trampoline when
// native operations finish immediately and VM continuations for Python calls.
func (call *sumCall) advance(caller *frame) (instructionOutcome, error) {
	for !call.done {
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := call.operation(caller)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			call.accept(result)
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
	return pushOutcome(caller, call.instruction, call.total)
}

func (call *sumCall) operation(caller *frame) (instructionOutcome, error) {
	if !call.adding {
		return executeBuiltinNext(caller, call.instruction, len(caller.stack), []Value{call.iterator, call.sentinel}, nil)
	}
	if call.mode == 1 {
		number, exception, numeric := numericFloat(call.item)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if numeric {
			return pushOutcome(caller, call.instruction, &floatValue{value: call.addFloat(number)})
		}
		call.mode = 2
	}
	return executeBinaryValues(caller, call.instruction, bytecode.BinaryAdd, false, call.total, call.item)
}

// accept changes accumulation mode only while leaving the native integer
// prefix; a general Python addition does not restart float compensation later.
func (call *sumCall) accept(result Value) {
	if !call.adding {
		if result == call.sentinel {
			call.done = true
			return
		}
		call.item, call.adding = result, true
		return
	}
	call.total, call.item, call.adding = result, nil, false
	if call.mode == 0 {
		if integer, ok := result.(*intValue); ok && integer.value.IsInt64() {
			return
		}
		call.mode = 2
		if number, ok := result.(*floatValue); ok {
			call.mode, call.high = 1, number.value
		}
	}
}

// addFloat tracks lost low-order bits with CPython's compensated summation,
// while preserving infinities and the sign of an uncompensated zero.
func (call *sumCall) addFloat(value float64) float64 {
	next := call.high + value
	if math.Abs(call.high) >= math.Abs(value) {
		call.low += (call.high - next) + value
	} else {
		call.low += (value - next) + call.high
	}
	call.high = next
	if call.low != 0 && !math.IsInf(call.low, 0) && !math.IsNaN(call.low) {
		return call.high + call.low
	}
	return call.high
}
