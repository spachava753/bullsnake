package runtime

import (
	"encoding/binary"
	"hash/fnv"
	"io"
	"math"
	"math/big"

	"github.com/spachava753/bullsnake/host"
)

const randomStateAttribute = "__bullsnake_random_state"

func (runtimeState *Runtime) newRandomBootstrapModule() *Module {
	module := newSystemModule("_random", "")
	randomType := newBootstrapClass("Random", nil)
	randomType.module = "_random"
	randomType.namespace.values["seed"] = nativeMethodNamed(
		"_random.Random.seed", 1, 2, runtimeState.randomSeed,
	)
	randomType.namespace.values["random"] = nativeMethodNamed(
		"_random.Random.random", 1, 1, randomNextFloat,
	)
	randomType.namespace.values["getrandbits"] = nativeMethodNamed(
		"_random.Random.getrandbits", 2, 2, randomGetBits,
	)
	randomType.namespace.values["getstate"] = nativeMethodNamed(
		"_random.Random.getstate", 1, 1, randomGetState,
	)
	randomType.namespace.values["setstate"] = nativeMethodNamed(
		"_random.Random.setstate", 2, 2, randomSetState,
	)
	module.globals.values["Random"] = randomType
	return module
}

// randomSeed initializes one Random instance from an explicit value or host
// entropy when Python requests the default seed.
func (runtimeState *Runtime) randomSeed(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	instance, ok := arguments[0].(*instanceValue)
	if !ok {
		return nil, newException("TypeError", "seed requires a Random instance"), nil
	}
	seed := uint64(0)
	if len(arguments) == 1 || arguments[1] == None {
		if runtimeState.host.Entropy == nil {
			return nil, hostFailure("random seed", host.ErrDenied), nil
		}
		buffer := make([]byte, 8)
		if _, err := io.ReadFull(runtimeState.host.Entropy, buffer); err != nil {
			return nil, hostFailure("random seed", err), nil
		}
		seed = binary.LittleEndian.Uint64(buffer)
	} else {
		seed = randomSeedValue(arguments[1])
	}
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	instance.attributes.values[randomStateAttribute] = unsignedInteger(seed)
	return None, nil, nil
}

// randomSeedValue deterministically folds the supported Python seed types into
// the generator's compact internal state.
func randomSeedValue(value Value) uint64 {
	if integer, ok := integerOperand(value); ok {
		return integer.Uint64()
	}
	switch value := value.(type) {
	case *floatValue:
		return math.Float64bits(value.value)
	case *stringValue:
		return hashRandomSeed(value.value)
	case *bytesValue:
		return hashRandomSeed(value.value)
	case *bytearrayValue:
		return hashRandomSeed(value.value)
	default:
		return hashRandomSeed(value.Repr())
	}
}

func hashRandomSeed(value string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))
	return hash.Sum64()
}

func randomNextFloat(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	instance, exception := randomInstance(arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	state := advanceRandomState(instance)
	return &floatValue{value: float64(state>>11) * (1.0 / (1 << 53))}, nil, nil
}

// randomGetBits advances the generator enough times to assemble an arbitrary
// non-negative number of random bits.
func randomGetBits(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	instance, exception := randomInstance(arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	count, ok := integerOperand(arguments[1])
	if !ok || !count.IsInt64() {
		return nil, newException("TypeError", "number of bits must be an integer"), nil
	}
	if count.Sign() < 0 {
		return nil, newException("ValueError", "number of bits must be non-negative"), nil
	}
	remaining := count.Int64()
	result := new(big.Int)
	for remaining > 0 {
		width := int64(64)
		if remaining < width {
			width = remaining
		}
		word := advanceRandomState(instance)
		if width < 64 {
			word &= (uint64(1) << width) - 1
		}
		result.Lsh(result, uint(width))
		result.Or(result, new(big.Int).SetUint64(word))
		remaining -= width
	}
	return &intValue{value: *result}, nil, nil
}

func randomGetState(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	instance, exception := randomInstance(arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	return unsignedInteger(currentRandomState(instance)), nil, nil
}

func randomSetState(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	instance, exception := randomInstance(arguments[0])
	if exception != nil {
		return nil, exception, nil
	}
	state, ok := integerOperand(arguments[1])
	if !ok {
		return nil, newException("TypeError", "state must be an integer"), nil
	}
	instance.attributes.values[randomStateAttribute] = unsignedInteger(state.Uint64())
	return None, nil, nil
}

func randomInstance(value Value) (*instanceValue, *Exception) {
	instance, ok := value.(*instanceValue)
	if !ok {
		return nil, newException("TypeError", "descriptor requires a Random instance")
	}
	return instance, nil
}

func currentRandomState(instance *instanceValue) uint64 {
	if value, found := instance.attributes.get(randomStateAttribute); found {
		if integer, ok := integerOperand(value); ok {
			return integer.Uint64()
		}
	}
	return 0x9e3779b97f4a7c15
}

func advanceRandomState(instance *instanceValue) uint64 {
	state := currentRandomState(instance)
	state ^= state >> 12
	state ^= state << 25
	state ^= state >> 27
	state *= 0x2545f4914f6cdd1d
	instance.attributes.values[randomStateAttribute] = unsignedInteger(state)
	return state
}

func unsignedInteger(value uint64) *intValue {
	return &intValue{value: *new(big.Int).SetUint64(value)}
}
