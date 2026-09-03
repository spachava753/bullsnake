package runtime

import "fmt"

type nativeFunction func(*frame, []Value) (Value, *Exception, error)

type nativeFunctionValue struct {
	name     string
	minimum  int
	maximum  int
	function nativeFunction
}

func (*nativeFunctionValue) TypeName() string { return "builtin_function_or_method" }
func (function *nativeFunctionValue) Repr() string {
	return "<built-in function " + function.name + ">"
}
func (*nativeFunctionValue) isValue() {}

// executeNativeFunctionCall applies the common arity and keyword contract,
// invokes one Go-backed function, and consumes the complete call segment.
func executeNativeFunctionCall(
	caller *frame,
	instruction int,
	base int,
	function *nativeFunctionValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				function.name+"() takes no keyword arguments",
			),
		}, nil
	}
	if len(arguments) < function.minimum ||
		(function.maximum >= 0 && len(arguments) > function.maximum) {
		return instructionOutcome{
			kind:      raised,
			exception: nativeArityError(function, len(arguments)),
		}, nil
	}
	value, exception, err := function.function(caller, arguments)
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if value == nil {
		return instructionOutcome{}, caller.failure(
			instruction,
			"native function returned no value",
		)
	}
	return pushOutcome(caller, instruction, value)
}

func nativeArityError(function *nativeFunctionValue, actual int) *Exception {
	expected := fmt.Sprintf("%d", function.minimum)
	if function.maximum < 0 {
		expected = fmt.Sprintf("at least %d", function.minimum)
	} else if function.minimum != function.maximum {
		expected = fmt.Sprintf("from %d to %d", function.minimum, function.maximum)
	}
	argument := "arguments"
	if function.minimum == function.maximum && function.minimum == 1 {
		argument = "argument"
	}
	return newException(
		"TypeError",
		fmt.Sprintf(
			"%s() takes %s positional %s but %d were given",
			function.name,
			expected,
			argument,
			actual,
		),
	)
}

func nativeFunctionNamed(
	name string,
	minimum int,
	maximum int,
	function nativeFunction,
) *nativeFunctionValue {
	return &nativeFunctionValue{
		name:     name,
		minimum:  minimum,
		maximum:  maximum,
		function: function,
	}
}
