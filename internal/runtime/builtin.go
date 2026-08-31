package runtime

import (
	"fmt"
	"math"
	"math/big"
)

type builtinFunctionValue struct {
	name      string
	call      func(arguments []Value, keywords *dictValue) (Value, *Exception)
	frameCall func(*frame, int, int, []Value, *dictValue) (instructionOutcome, error)
}

func (*builtinFunctionValue) TypeName() string { return "builtin_function_or_method" }
func (function *builtinFunctionValue) Repr() string {
	return "<built-in function " + function.name + ">"
}
func (*builtinFunctionValue) isValue() {}

func executeBuiltinFunctionCall(
	caller *frame,
	instruction int,
	base int,
	function *builtinFunctionValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if function.frameCall != nil {
		return function.frameCall(caller, instruction, base, arguments, keywords)
	}
	result, exception := function.call(arguments, keywords)
	discardCallSegment(caller, base)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	return pushOutcome(caller, instruction, result)
}

func discardCallSegment(caller *frame, base int) {
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
}

var builtinFunctions = []*builtinFunctionValue{
	{name: "int", call: builtinInt},
	{name: "max", call: builtinMax},
	{name: "min", call: builtinMin},
	{name: "next", frameCall: executeBuiltinNext},
	{name: "property", call: builtinProperty},
}

// builtinInt converts the currently supported scalar numeric values.
func builtinInt(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if keywords != nil && len(keywords.entries) != 0 {
		return nil, newException("TypeError", "int() keyword arguments are unsupported")
	}
	if len(arguments) != 1 {
		return nil, newException("TypeError", "int() requires exactly one argument")
	}
	switch value := arguments[0].(type) {
	case *intValue:
		return value, nil
	case *boolValue:
		var integer big.Int
		if value.value {
			integer.SetInt64(1)
		}
		return &intValue{value: integer}, nil
	case *floatValue:
		if math.IsInf(value.value, 0) {
			return nil, newException(
				"OverflowError",
				"cannot convert float infinity to integer",
			)
		}
		if math.IsNaN(value.value) {
			return nil, newException(
				"ValueError",
				"cannot convert float NaN to integer",
			)
		}
		integer, _ := new(big.Float).SetFloat64(value.value).Int(nil)
		return &intValue{value: *integer}, nil
	default:
		return nil, newException(
			"TypeError",
			"int() argument must be a real number, not '"+value.TypeName()+"'",
		)
	}
}

func builtinMax(arguments []Value, keywords *dictValue) (Value, *Exception) {
	return positionalExtremum("max", arguments, keywords, true)
}

func builtinMin(arguments []Value, keywords *dictValue) (Value, *Exception) {
	return positionalExtremum("min", arguments, keywords, false)
}

// positionalExtremum selects one ordered positional value for max or min.
func positionalExtremum(
	name string,
	arguments []Value,
	keywords *dictValue,
	greatest bool,
) (Value, *Exception) {
	if keywords != nil && len(keywords.entries) != 0 {
		return nil, newException("TypeError", name+"() keyword arguments are unsupported")
	}
	if len(arguments) == 0 {
		return nil, newException(
			"TypeError",
			fmt.Sprintf("%s expected at least 1 argument, got 0", name),
		)
	}
	if len(arguments) == 1 {
		return nil, newException("TypeError", name+"() single-argument form is unsupported")
	}
	selected := arguments[0]
	for _, candidate := range arguments[1:] {
		comparison, ordered, supported := orderedValues(candidate, selected)
		if !supported {
			operator := "<"
			if greatest {
				operator = ">"
			}
			return nil, newException(
				"TypeError",
				fmt.Sprintf(
					"'%s' not supported between instances of '%s' and '%s'",
					operator,
					candidate.TypeName(),
					selected.TypeName(),
				),
			)
		}
		if ordered && ((greatest && comparison > 0) || (!greatest && comparison < 0)) {
			selected = candidate
		}
	}
	return selected, nil
}
