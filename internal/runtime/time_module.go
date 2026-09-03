package runtime

import (
	"math"
	"math/big"
	"time"

	"github.com/spachava753/bullsnake/host"
)

func (runtime *Runtime) newTimeModule() *Module {
	module := newSystemModule("time", "")
	setNativeFunction(module, "time", 0, 0, runtime.timeNow)
	setNativeFunction(module, "monotonic", 0, 0, runtime.monotonicFunction("time.monotonic"))
	setNativeFunction(module, "perf_counter", 0, 0, runtime.monotonicFunction("time.perf_counter"))
	setNativeFunction(module, "sleep", 1, 1, runtime.sleep)
	return module
}

func (runtime *Runtime) timeNow(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	if runtime.host.Clock == nil {
		return nil, hostFailure("time.time", host.ErrDenied), nil
	}
	now, err := runtime.host.Clock.Now()
	if err != nil {
		return nil, hostFailure("time.time", err), nil
	}
	seconds := float64(now.Unix()) + float64(now.Nanosecond())/float64(time.Second)
	return &floatValue{value: seconds}, nil, nil
}

func (runtime *Runtime) monotonicFunction(operation string) nativeFunction {
	return func(_ *frame, _ []Value) (Value, *Exception, error) {
		if runtime.host.Clock == nil {
			return nil, hostFailure(operation, host.ErrDenied), nil
		}
		duration, err := runtime.host.Clock.Monotonic()
		if err != nil {
			return nil, hostFailure(operation, err), nil
		}
		return &floatValue{value: duration.Seconds()}, nil, nil
	}
}

// sleep validates finite non-negative seconds before delegating all waiting to
// the configured clock capability.
func (runtime *Runtime) sleep(
	_ *frame,
	arguments []Value,
) (Value, *Exception, error) {
	seconds, ok := numericFloat(arguments[0])
	if !ok {
		return nil, newException(
			"TypeError",
			"'"+arguments[0].TypeName()+"' object cannot be interpreted as a number",
		), nil
	}
	if math.IsNaN(seconds) {
		return nil, newException("ValueError", "Invalid value NaN (not a number)"), nil
	}
	if seconds < 0 {
		return nil, newException("ValueError", "sleep length must be non-negative"), nil
	}
	if math.IsInf(seconds, 0) || seconds > float64(math.MaxInt64)/float64(time.Second) {
		return nil, newException("OverflowError", "timestamp out of range for platform time_t"), nil
	}
	if runtime.host.Clock == nil {
		return nil, hostFailure("time.sleep", host.ErrDenied), nil
	}
	if err := runtime.host.Clock.Sleep(time.Duration(seconds * float64(time.Second))); err != nil {
		return nil, hostFailure("time.sleep", err), nil
	}
	return None, nil, nil
}

func numericFloat(value Value) (float64, bool) {
	switch value := value.(type) {
	case *floatValue:
		return value.value, true
	case *intValue:
		converted, _ := new(big.Float).SetInt(&value.value).Float64()
		return converted, true
	case *boolValue:
		if value.value {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}
