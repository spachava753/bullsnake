package runtime

import "math"

func newMathModule() *Module {
	module := newSystemModule("math", "")
	module.globals.values["e"] = &floatValue{value: math.E}
	module.globals.values["pi"] = &floatValue{value: math.Pi}
	module.globals.values["tau"] = &floatValue{value: 2 * math.Pi}
	module.globals.values["inf"] = &floatValue{value: math.Inf(1)}
	module.globals.values["nan"] = &floatValue{value: math.NaN()}
	setNativeFunction(module, "modf", 1, 1, mathModf)
	setNativeFunction(module, "copysign", 2, 2, mathCopysign)
	setNativeFunction(module, "isnan", 1, 1, mathIsNaN)
	for name, function := range map[string]func(float64) float64{
		"log": math.Log, "exp": math.Exp, "sqrt": math.Sqrt, "acos": math.Acos,
		"cos": math.Cos, "sin": math.Sin, "fabs": math.Abs,
		"log2": math.Log2,
	} {
		setNativeFunction(module, name, 1, 1, mathUnary(function))
	}
	setNativeFunction(module, "lgamma", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			value, ok := numericFloat(arguments[0])
			if !ok {
				return nil, newException("TypeError", "must be real number"), nil
			}
			result, _ := math.Lgamma(value)
			return &floatValue{value: result}, nil, nil
		})
	setNativeFunction(module, "ceil", 1, 1, mathIntegral(math.Ceil))
	setNativeFunction(module, "floor", 1, 1, mathIntegral(math.Floor))
	setNativeFunction(module, "trunc", 1, 1, mathIntegralSpecial("__trunc__", math.Trunc))
	module.globals.values["ceil"] = nativeFunctionNamed("math.ceil", 1, 1, mathIntegralSpecial("__ceil__", math.Ceil))
	module.globals.values["floor"] = nativeFunctionNamed("math.floor", 1, 1, mathIntegralSpecial("__floor__", math.Floor))
	setNativeFunction(module, "isfinite", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			value, ok := numericFloat(arguments[0])
			if !ok {
				return nil, newException("TypeError", "must be real number"), nil
			}
			return pythonBool(!math.IsInf(value, 0) && !math.IsNaN(value)), nil, nil
		})
	return module
}

func mathUnary(function func(float64) float64) nativeFunction {
	return func(_ *frame, arguments []Value) (Value, *Exception, error) {
		value, ok := numericFloat(arguments[0])
		if !ok {
			return nil, newException("TypeError", "must be real number"), nil
		}
		return &floatValue{value: function(value)}, nil, nil
	}
}

func mathIntegral(function func(float64) float64) nativeFunction {
	return func(_ *frame, arguments []Value) (Value, *Exception, error) {
		value, ok := numericFloat(arguments[0])
		if !ok {
			return nil, newException("TypeError", "must be real number"), nil
		}
		return newInt64(int64(function(value))), nil, nil
	}
}

// mathIntegralSpecial constructs floor-like functions that accept native
// numbers or dispatch the corresponding special method on user instances.
func mathIntegralSpecial(name string, function func(float64) float64) nativeFunction {
	return func(caller *frame, arguments []Value) (Value, *Exception, error) {
		if value, ok := numericFloat(arguments[0]); ok {
			return newInt64(int64(function(value))), nil, nil
		}
		if instance, ok := arguments[0].(*instanceValue); ok {
			method, found, exception, err := lookupBoundSpecialMethod(caller, instance, name)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if found {
				return callValueSynchronously(caller, method, nil)
			}
		}
		return nil, newException("TypeError", "must be real number"), nil
	}
}

func mathModf(_ *frame, arguments []Value) (Value, *Exception, error) {
	value, ok := numericFloat(arguments[0])
	if !ok {
		return nil, newException("TypeError", "must be real number"), nil
	}
	integer, fraction := math.Modf(value)
	return &tupleValue{elements: []Value{
		&floatValue{value: fraction},
		&floatValue{value: integer},
	}}, nil, nil
}

func mathCopysign(_ *frame, arguments []Value) (Value, *Exception, error) {
	magnitude, magnitudeOK := numericFloat(arguments[0])
	sign, signOK := numericFloat(arguments[1])
	if !magnitudeOK || !signOK {
		return nil, newException("TypeError", "must be real number"), nil
	}
	return &floatValue{value: math.Copysign(magnitude, sign)}, nil, nil
}

func mathIsNaN(_ *frame, arguments []Value) (Value, *Exception, error) {
	value, ok := numericFloat(arguments[0])
	if !ok {
		return nil, newException("TypeError", "must be real number"), nil
	}
	return pythonBool(math.IsNaN(value)), nil, nil
}
