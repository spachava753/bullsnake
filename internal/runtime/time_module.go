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
	setNativeFunction(module, "time_ns", 0, 0, runtime.timeNowNS)
	setNativeFunction(module, "monotonic", 0, 0, runtime.monotonicFunction("time.monotonic"))
	setNativeFunction(module, "perf_counter", 0, 0, runtime.monotonicFunction("time.perf_counter"))
	setNativeFunction(module, "sleep", 1, 1, runtime.sleep)
	setNativeFunction(module, "gmtime", 0, 1, runtime.timeTuple(false))
	setNativeFunction(module, "localtime", 0, 1, runtime.timeTuple(true))
	setNativeFunction(module, "ctime", 0, 1, runtime.ctime)
	setNativeFunction(module, "strftime", 1, 2,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			if text, ok := arguments[0].(*stringValue); ok {
				return text, nil, nil
			}
			return nil, newException("TypeError", "format must be str"), nil
		})
	return module
}

// ctime obtains or converts one instant, delegates local civil-time conversion
// to the host, and renders the traditional fixed-width timestamp.
func (runtime *Runtime) ctime(_ *frame, arguments []Value) (Value, *Exception, error) {
	var instant time.Time
	if len(arguments) == 0 {
		if runtime.host.Clock == nil {
			return nil, hostFailure("time.ctime", host.ErrDenied), nil
		}
		value, err := runtime.host.Clock.Now()
		if err != nil {
			return nil, hostFailure("time.ctime", err), nil
		}
		instant = value
	} else {
		seconds, ok := numericFloat(arguments[0])
		if !ok {
			return nil, newException("TypeError", "an integer is required"), nil
		}
		whole, fraction := math.Modf(seconds)
		instant = time.Unix(int64(whole), int64(fraction*float64(time.Second)))
	}
	if runtime.host.TimeZone == nil {
		return nil, hostFailure("time.ctime", host.ErrDenied), nil
	}
	instant, err := runtime.host.TimeZone.LocalTime(instant)
	if err != nil {
		return nil, hostFailure("time.ctime", err), nil
	}
	return &stringValue{value: instant.Format(time.ANSIC)}, nil, nil
}

// timeTuple converts an explicit or host-provided instant to Python's nine-item tuple.
func (runtime *Runtime) timeTuple(local bool) nativeFunction {
	return func(_ *frame, arguments []Value) (Value, *Exception, error) {
		var instant time.Time
		if len(arguments) == 0 {
			if runtime.host.Clock == nil {
				return nil, hostFailure("time.localtime", host.ErrDenied), nil
			}
			value, err := runtime.host.Clock.Now()
			if err != nil {
				return nil, hostFailure("time.localtime", err), nil
			}
			instant = value
		} else {
			seconds, ok := numericFloat(arguments[0])
			if !ok {
				return nil, newException("TypeError", "timestamp must be a real number"), nil
			}
			whole, fraction := math.Modf(seconds)
			instant = time.Unix(int64(whole), int64(fraction*float64(time.Second)))
		}
		if !local {
			instant = instant.UTC()
		} else {
			if runtime.host.TimeZone == nil {
				return nil, hostFailure("time.localtime", host.ErrDenied), nil
			}
			localized, err := runtime.host.TimeZone.LocalTime(instant)
			if err != nil {
				return nil, hostFailure("time.localtime", err), nil
			}
			instant = localized
		}
		weekday := (int(instant.Weekday()) + 6) % 7
		return &tupleValue{elements: []Value{
			newInt64(int64(instant.Year())),
			newInt64(int64(instant.Month())),
			newInt64(int64(instant.Day())),
			newInt64(int64(instant.Hour())),
			newInt64(int64(instant.Minute())),
			newInt64(int64(instant.Second())),
			newInt64(int64(weekday)),
			newInt64(int64(instant.YearDay())),
			newInt64(0),
		}}, nil, nil
	}
}

func (runtime *Runtime) timeNowNS(_ *frame, _ []Value) (Value, *Exception, error) {
	if runtime.host.Clock == nil {
		return nil, hostFailure("time.time_ns", host.ErrDenied), nil
	}
	now, err := runtime.host.Clock.Now()
	if err != nil {
		return nil, hostFailure("time.time_ns", err), nil
	}
	return newInt64(now.UnixNano()), nil, nil
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
