package runtime

import "fmt"

type nativeFunction func(*frame, []Value) (Value, *Exception, error)

type nativeKeywordFunction func(*frame, []Value, *dictValue) (Value, *Exception, error)

type nativeTypeConstructor func(*typeValue, *frame, []Value, *dictValue) (Value, *Exception, error)

type nativeFunctionValue struct {
	name            string
	minimum         int
	maximum         int
	function        nativeFunction
	keywordFunction nativeKeywordFunction
	keywords        bool
	bindReceiver    bool
	attributes      *Namespace
}

func (*nativeFunctionValue) TypeName() string { return "builtin_function_or_method" }
func (function *nativeFunctionValue) Repr() string {
	return "<built-in function " + function.name + ">"
}
func (*nativeFunctionValue) isValue() {}

// attribute exposes metadata and writable attributes for a Go-backed function.
func (function *nativeFunctionValue) attribute(name string) (Value, bool) {
	switch name {
	case "__name__", "__qualname__":
		return &stringValue{value: function.name}, true
	case "__module__", "__doc__", "__annotate__":
		return None, true
	case "__type_params__":
		return &tupleValue{}, true
	case "__call__":
		return function, true
	case "__dict__":
		if function.attributes == nil {
			function.attributes = newNamespace()
		}
		return &namespaceValue{namespace: function.attributes}, true
	}
	if function.attributes == nil {
		return nil, false
	}
	return function.attributes.get(name)
}

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
	if !function.keywords && keywords != nil && len(keywords.entries) != 0 {
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
	var value Value
	var exception *Exception
	var err error
	if function.keywordFunction != nil {
		value, exception, err = function.keywordFunction(caller, arguments, keywords)
	} else {
		value, exception, err = function.function(caller, arguments)
	}
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

func nativeKeywordFunctionNamed(
	name string,
	minimum int,
	maximum int,
	function nativeFunction,
) *nativeFunctionValue {
	value := nativeFunctionNamed(name, minimum, maximum, function)
	value.keywords = true
	return value
}

func nativeKeywordAwareFunctionNamed(
	name string,
	minimum int,
	maximum int,
	function nativeKeywordFunction,
) *nativeFunctionValue {
	return &nativeFunctionValue{
		name:            name,
		minimum:         minimum,
		maximum:         maximum,
		keywordFunction: function,
		keywords:        true,
	}
}

// executeNativeTypeCall applies the shared keyword contract and invokes a
// Go-backed constructor while consuming its complete caller stack segment.
func executeNativeTypeCall(
	caller *frame,
	instruction int,
	base int,
	class *typeValue,
	constructor nativeTypeConstructor,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	value, exception, err := constructor(class, caller, arguments, keywords)
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

// nativeMethodNamed creates a Go-backed descriptor stored on a type. Module
// builtins deliberately do not bind when a Python class merely assigns one to
// a class attribute (for example, “handler = signal.default_int_handler“).
func nativeMethodNamed(
	name string,
	minimum int,
	maximum int,
	function nativeFunction,
) *nativeFunctionValue {
	value := nativeFunctionNamed(name, minimum, maximum, function)
	value.bindReceiver = true
	return value
}
