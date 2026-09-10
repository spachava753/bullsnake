package runtime

import (
	"hash/fnv"
	"math"
	"math/big"
	"strconv"
)

type hashCall struct {
	instruction int
}

var hashModulus = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 61), big.NewInt(1))

// executeBuiltinHash returns fixed hashes immediately and suspends for a user
// instance's class __hash__ method through the ordinary frame loop.
func executeBuiltinHash(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"hash() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"hash() takes exactly one argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)), nil
	}
	value := arguments[0]
	discardCallSegment(caller, base)
	if instance, ok := value.(*instanceValue); ok {
		return executeUserHash(caller, instruction, instance)
	}
	hash, exception, supported := fixedValueHash(value)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if !supported {
		return raiseOutcome(newException(
			"NotImplementedError",
			"hashing '"+value.TypeName()+"' is not supported",
		)), nil
	}
	return pushOutcome(caller, instruction, hashIntegerValue(hash))
}

// executeUserHash applies an enabled class __hash__ method, falls back to a
// stable identity-style hash, and preserves Python's disabled-slot error.
func executeUserHash(
	frame *frame,
	instruction int,
	instance *instanceValue,
) (instructionOutcome, error) {
	method, found := lookupInstanceSpecial(instance, "__hash__")
	if !found {
		hash := stableTextHash(instance.TypeName(), instance.Repr())
		return pushOutcome(frame, instruction, hashIntegerValue(hash))
	}
	if method == None {
		return raiseOutcome(unhashableTypeError(instance.TypeName())), nil
	}
	call := &hashCall{instruction: instruction}
	outcome, err := executeFunctionCall(
		frame,
		instruction,
		len(frame.stack),
		method,
		nil,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.hash = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"hash special method returned without a value",
		)
	}
	return finishHashCall(frame, call, result)
}

func finishHashCall(
	frame *frame,
	call *hashCall,
	result Value,
) (instructionOutcome, error) {
	integer, ok := integerOperand(result)
	if !ok {
		return raiseOutcome(newException(
			"TypeError",
			"__hash__ method should return an integer",
		)), nil
	}
	return pushOutcome(frame, call.instruction, hashIntegerValue(hashBigInteger(&integer)))
}

// fixedValueHash preserves the equality contract for current scalar values and
// recursively hashes immutable built-in collections without invoking Python.
func fixedValueHash(value Value) (int64, *Exception, bool) {
	if integer, ok := integerOperand(value); ok {
		return hashBigInteger(&integer), nil, true
	}
	switch value := value.(type) {
	case *classWeakReference:
		return value.hash, nil, true
	case *noneValue:
		return 0x27d4eb2d, nil, true
	case *floatValue:
		return hashFloat(value.value), nil, true
	case *complexValue:
		real := hashFloat(value.real)
		if value.imaginary == 0 {
			return real, nil, true
		}
		imaginary := hashFloat(value.imaginary)
		return normalizeHashInt64(real + 1000003*imaginary), nil, true
	case *stringValue:
		return stableTextHash("str", value.value), nil, true
	case *bytesValue:
		return stableTextHash("bytes", value.value), nil, true
	case *tupleValue:
		return hashTuple(value.elements)
	case *frozenSetValue:
		return hashFrozenSet(value.entries)
	case *rangeValue:
		parts := []Value{
			&intValue{value: *new(big.Int).Set(&value.start)},
			&intValue{value: *new(big.Int).Set(&value.stop)},
			&intValue{value: *new(big.Int).Set(&value.step)},
		}
		hash, _, _ := hashTuple(parts)
		return hash, nil, true
	case *listValue, *dictValue, *setValue, *bytearrayValue:
		return 0, unhashableTypeError(value.TypeName()), true
	case *sliceValue:
		return 0, unhashableTypeError(value.TypeName()), true
	case *instanceValue:
		return 0, nil, false
	default:
		return stableTextHash(value.TypeName(), value.Repr()), nil, true
	}
}

func hashTuple(elements []Value) (int64, *Exception, bool) {
	accumulator := big.NewInt(0x345678)
	multiplier := big.NewInt(1000003)
	for _, element := range elements {
		hash, exception, supported := fixedValueHash(element)
		if exception != nil {
			return 0, exception, true
		}
		if !supported {
			return 0, newException(
				"NotImplementedError",
				"hashing tuples containing user values is not supported",
			), true
		}
		accumulator.Mul(accumulator, multiplier)
		accumulator.Add(accumulator, big.NewInt(hash))
	}
	accumulator.Add(accumulator, big.NewInt(97531))
	return hashBigInteger(accumulator), nil, true
}

func hashFrozenSet(elements []Value) (int64, *Exception, bool) {
	accumulator := big.NewInt(1927868237)
	for _, element := range elements {
		hash, exception, supported := fixedValueHash(element)
		if exception != nil {
			return 0, exception, true
		}
		if !supported {
			return 0, newException(
				"NotImplementedError",
				"hashing frozen sets containing user values is not supported",
			), true
		}
		accumulator.Add(accumulator, big.NewInt(hash))
	}
	return hashBigInteger(accumulator), nil, true
}

func hashFloat(value float64) int64 {
	if !math.IsNaN(value) && !math.IsInf(value, 0) && math.Trunc(value) == value {
		integer, _ := new(big.Float).SetFloat64(value).Int(nil)
		return hashBigInteger(integer)
	}
	bits := new(big.Int).SetUint64(math.Float64bits(value))
	return hashBigInteger(bits)
}

func stableTextHash(kind, value string) int64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(kind))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(value))
	integer := new(big.Int).SetUint64(hasher.Sum64())
	return hashBigInteger(integer)
}

func hashBigInteger(integer *big.Int) int64 {
	magnitude := new(big.Int).Abs(new(big.Int).Set(integer))
	magnitude.Mod(magnitude, hashModulus)
	if integer.Sign() < 0 {
		magnitude.Neg(magnitude)
	}
	return normalizeHashInt64(magnitude.Int64())
}

func normalizeHashInt64(hash int64) int64 {
	if hash == -1 {
		return -2
	}
	return hash
}

func hashIntegerValue(hash int64) *intValue {
	var integer big.Int
	integer.SetInt64(hash)
	return &intValue{value: integer}
}

func unhashableTypeError(typeName string) *Exception {
	return newException("TypeError", "unhashable type: '"+typeName+"'")
}
