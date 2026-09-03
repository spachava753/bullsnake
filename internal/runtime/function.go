package runtime

import (
	"fmt"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type functionValue struct {
	code            *preparedCode
	globals         *Namespace
	defaults        []Value
	keywordDefaults map[string]Value
	closure         []*cellValue
	annotate        *functionValue
	doc             Value
	attributes      *Namespace
}

func (*functionValue) TypeName() string { return "function" }
func (function *functionValue) Repr() string {
	return "<function " + function.code.code.QualifiedName() + ">"
}
func (*functionValue) isValue() {}

// attribute exposes function metadata together with its writable attribute namespace.
func (function *functionValue) attribute(name string) (Value, bool) {
	if function.attributes != nil && name != "__dict__" {
		if value, found := function.attributes.get(name); found {
			return value, true
		}
	}
	switch name {
	case "__name__":
		qualified := function.code.code.QualifiedName()
		if index := strings.LastIndexByte(qualified, '.'); index >= 0 {
			qualified = qualified[index+1:]
		}
		return &stringValue{value: qualified}, true
	case "__qualname__":
		return &stringValue{value: function.code.code.QualifiedName()}, true
	case "__module__":
		if module, found := function.globals.get("__name__"); found {
			return module, true
		}
		return None, true
	case "__doc__":
		if function.doc != nil {
			return function.doc, true
		}
		return None, true
	case "__type_params__":
		return &tupleValue{}, true
	case "__annotations__":
		return &dictValue{}, true
	case "__annotate__":
		if function.annotate != nil {
			return function.annotate, true
		}
		return None, true
	case "__closure__":
		elements := make([]Value, len(function.closure))
		for index, cell := range function.closure {
			elements[index] = cell
		}
		return &tupleValue{elements: elements}, true
	case "__defaults__":
		if len(function.defaults) == 0 {
			return None, true
		}
		return &tupleValue{elements: append([]Value(nil), function.defaults...)}, true
	case "__kwdefaults__":
		if len(function.keywordDefaults) == 0 {
			return None, true
		}
		defaults := &dictValue{}
		for name, value := range function.keywordDefaults {
			defaults.entries = append(defaults.entries, dictEntry{
				key: &stringValue{value: name}, value: value,
			})
		}
		return defaults, true
	case "__globals__":
		return &namespaceValue{namespace: function.globals}, true
	case "__builtins__":
		if builtins, found := function.globals.get("__builtins__"); found {
			return builtins, true
		}
		return &namespaceValue{namespace: function.globals}, true
	case "__call__":
		return function, true
	case "__dict__":
		if function.attributes == nil {
			function.attributes = newNamespace()
		}
		return &namespaceValue{namespace: function.attributes}, true
	case "__code__":
		return &codeValue{code: function.code}, true
	}
	if function.attributes == nil {
		return nil, false
	}
	return function.attributes.get(name)
}

type codeValue struct {
	code *preparedCode
}

func (*codeValue) TypeName() string { return "code" }
func (*codeValue) Repr() string     { return "<code object>" }
func (*codeValue) isValue()         {}

// attribute exposes source metadata and the position iterator expected by
// traceback formatting without presenting Bullsnake offsets as CPython ones.
func (code *codeValue) attribute(name string) (Value, bool) {
	switch name {
	case "co_argcount":
		return newInt64(int64(code.code.code.PositionalCount())), true
	case "co_posonlyargcount":
		return newInt64(int64(code.code.code.PositionalOnlyCount())), true
	case "co_kwonlyargcount":
		return newInt64(int64(code.code.code.KeywordOnlyCount())), true
	case "co_varnames":
		locals := code.code.code.Locals()
		values := make([]Value, len(locals))
		for index, name := range locals {
			values[index] = &stringValue{value: name}
		}
		return &tupleValue{elements: values}, true
	case "co_freevars":
		names := code.code.code.FreeVars()
		values := make([]Value, len(names))
		for index, name := range names {
			values[index] = &stringValue{value: name}
		}
		return &tupleValue{elements: values}, true
	case "co_cellvars":
		names := code.code.code.Cells()
		values := make([]Value, len(names))
		for index, name := range names {
			values[index] = &stringValue{value: name}
		}
		return &tupleValue{elements: values}, true
	case "co_flags":
		return newInt64(int64(code.code.code.Flags())), true
	case "co_name":
		qualified := code.code.code.QualifiedName()
		if index := strings.LastIndexByte(qualified, '.'); index >= 0 {
			qualified = qualified[index+1:]
		}
		return &stringValue{value: qualified}, true
	case "co_filename":
		return &stringValue{value: code.code.code.Filename()}, true
	case "co_firstlineno":
		return newInt64(int64(code.code.code.FirstLine())), true
	case "co_positions":
		return nativeFunctionNamed("code.co_positions", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &sequenceIterator{sequence: &listValue{}}, nil, nil
			}), true
	default:
		return nil, false
	}
}

var _ Value = (*codeValue)(nil)

// executeCall removes one inline function and positional argument segment before
// delegating to the shared function-frame constructor.
func executeCall(
	caller *frame,
	instruction int,
	argumentCount int,
) (instructionOutcome, error) {
	required := argumentCount + 1
	if argumentCount < 0 || required > len(caller.stack) {
		return instructionOutcome{}, caller.failure(instruction, "operand stack underflow")
	}
	base := len(caller.stack) - required
	return executeFunctionCall(
		caller,
		instruction,
		base,
		caller.stack[base],
		caller.stack[base+1:],
		nil,
	)
}

// executeUnpackedCall validates the positional tuple and optional keyword map
// selected by CALL_EX before delegating to the shared function-frame path.
func executeUnpackedCall(
	caller *frame,
	instruction int,
	withKeywords bool,
) (instructionOutcome, error) {
	required := 2
	if withKeywords {
		required++
	}
	if len(caller.stack) < required {
		return instructionOutcome{}, caller.failure(instruction, "operand stack underflow")
	}
	base := len(caller.stack) - required
	arguments, ok := caller.stack[base+1].(*tupleValue)
	if !ok {
		return instructionOutcome{}, caller.failure(
			instruction,
			"CALL_EX positional arguments are not a tuple",
		)
	}
	var keywords *dictValue
	if withKeywords {
		keywords, ok = caller.stack[base+2].(*dictValue)
		if !ok {
			return instructionOutcome{}, caller.failure(
				instruction,
				"CALL_EX keyword arguments are not a dictionary",
			)
		}
	}
	return executeFunctionCall(
		caller,
		instruction,
		base,
		caller.stack[base],
		arguments.elements,
		keywords,
	)
}

// executeFunctionCall validates one callable, binds positional values into a
// fresh local array, consumes the caller segment, and creates the child frame.
func executeFunctionCall(
	caller *frame,
	instruction int,
	base int,
	callable Value,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	switch callable := callable.(type) {
	case *nativeFunctionValue:
		return executeNativeFunctionCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *buildClassValue:
		return executeBuildClassCall(
			caller,
			instruction,
			base,
			arguments,
			keywords,
		)
	case *typeValue:
		return executeTypeCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *exceptionTypeValue:
		return executeExceptionTypeCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *builtinTypeValue:
		return executeBuiltinTypeCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *boundMethodValue:
		boundArguments := make([]Value, len(arguments)+1)
		boundArguments[0] = callable.self
		copy(boundArguments[1:], arguments)
		return executeFunctionCall(
			caller,
			instruction,
			base,
			callable.function,
			boundArguments,
			keywords,
		)
	case *boundNativeMethodValue:
		boundArguments := make([]Value, len(arguments)+1)
		boundArguments[0] = callable.self
		copy(boundArguments[1:], arguments)
		return executeFunctionCall(
			caller,
			instruction,
			base,
			callable.function,
			boundArguments,
			keywords,
		)
	case *genericBoundMethodValue:
		boundArguments := make([]Value, len(arguments)+1)
		boundArguments[0] = callable.self
		copy(boundArguments[1:], arguments)
		return executeFunctionCall(
			caller, instruction, base, callable.callable, boundArguments, keywords,
		)
	case *instanceValue:
		method, found := callable.class.lookup("__call__")
		if !found {
			break
		}
		if resolved, descriptor, exception, err := resolvePythonDescriptor(
			caller, method, callable, callable.class,
		); descriptor {
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			method = resolved
		}
		return executeFunctionCall(
			caller,
			instruction,
			base,
			bindCallable(method, callable),
			arguments,
			keywords,
		)
	case *weakReferenceValue:
		if len(arguments) != 0 || (keywords != nil && len(keywords.entries) != 0) {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "weak reference takes no arguments",
			)}, nil
		}
		for index := base; index < len(caller.stack); index++ {
			caller.stack[index] = nil
		}
		caller.stack = caller.stack[:base]
		return pushOutcome(caller, instruction, callable.referent)
	case *partialValue:
		combined := append([]Value(nil), callable.arguments...)
		combined = append(combined, arguments...)
		merged := &dictValue{}
		if callable.keywords != nil {
			for _, entry := range callable.keywords.entries {
				if exception := merged.set(entry.key, entry.value); exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
			}
		}
		if keywords != nil {
			for _, entry := range keywords.entries {
				if exception := merged.set(entry.key, entry.value); exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
			}
		}
		return executeFunctionCall(
			caller, instruction, base, callable.callable, combined, merged,
		)
	}
	function, ok := callable.(*functionValue)
	if !ok {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+callable.TypeName()+"' object is not callable",
			),
		}, nil
	}
	locals, exception := bindFunctionArguments(function, arguments, keywords)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	deref, ok := initializeDeref(function.code, locals, function.closure)
	if !ok {
		return instructionOutcome{}, caller.failure(
			instruction,
			fmt.Sprintf(
				"function closure has %d cells for %d free variables",
				len(function.closure),
				len(function.code.freeVars),
			),
		)
	}
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	child := &frame{
		runtime:    caller.runtime,
		code:       function.code,
		stack:      make([]Value, 0, function.code.stackSize),
		fastLocals: locals,
		deref:      deref,
		locals:     newNamespace(),
		globals:    function.globals,
		builtins:   caller.builtins,
		previous:   caller,
	}
	if function.code.code.Flags()&bytecode.Generator != 0 {
		child.previous = nil
		generator := &generatorValue{frame: child}
		child.generator = generator
		return pushOutcome(caller, instruction, generator)
	}
	if function.code.code.Flags()&bytecode.Coroutine != 0 {
		child.previous = nil
		coroutine := &coroutineValue{frame: child, name: function.code.code.QualifiedName()}
		child.coroutine = coroutine
		caller.runtime.coroutines = append(caller.runtime.coroutines, coroutine)
		return pushOutcome(caller, instruction, coroutine)
	}
	if function.code.code.Flags()&bytecode.AsyncGenerator != 0 {
		child.previous = nil
		generator := &asyncGeneratorValue{frame: child, name: function.code.code.QualifiedName()}
		child.asyncGenerator = generator
		return pushOutcome(caller, instruction, generator)
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

var nativeCallbackCode = bytecode.NewCode(bytecode.CodeSpec{
	Name:          "<native callback>",
	QualifiedName: "<native callback>",
	StackSize:     1,
	Instructions: []bytecode.Instruction{
		{Opcode: bytecode.LoadConst},
		{Opcode: bytecode.ReturnValue},
	},
	Positions: make([]lexer.Span, 2),
	Constants: []bytecode.Constant{bytecode.None()},
})

// callValueSynchronously lets an eager native operation invoke any callable
// already supported by ordinary CALL dispatch and collect its result.
func callValueSynchronously(
	caller *frame,
	callable Value,
	arguments []Value,
) (Value, *Exception, error) {
	return callValueSynchronouslyWithKeywords(caller, callable, arguments, nil)
}

// callValueSynchronouslyWithKeywords executes native or Python callables in a
// synthetic validated frame and returns either their value or Python error.
func callValueSynchronouslyWithKeywords(
	caller *frame,
	callable Value,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	prepared, err := caller.runtime.prepare(nativeCallbackCode)
	if err != nil {
		return nil, nil, err
	}
	callback := &frame{
		runtime:         caller.runtime,
		code:            prepared,
		instruction:     1,
		stack:           append([]Value{callable}, arguments...),
		locals:          newNamespace(),
		globals:         caller.globals,
		builtins:        caller.builtins,
		logicalPrevious: caller,
	}
	outcome, err := executeFunctionCall(
		callback, 0, 0, callable, arguments, keywords,
	)
	if err != nil {
		return nil, nil, err
	}
	switch outcome.kind {
	case raised:
		return nil, outcome.exception, nil
	case called:
		result, unhandled, executeErr := execute(&threadState{current: outcome.frame})
		if executeErr != nil {
			return nil, nil, executeErr
		}
		if unhandled != nil {
			return nil, unhandled.exception, nil
		}
		return result, nil, nil
	case advance:
		value, ok := callback.pop()
		if !ok {
			return nil, nil, callback.failure(0, "native callback returned no value")
		}
		return value, nil, nil
	default:
		return nil, nil, callback.failure(0, "invalid native callback outcome")
	}
}

func keywordOnlyRange(code *preparedCode) (int, int) {
	start := code.code.PositionalCount()
	if code.code.Flags()&bytecode.VarArgs != 0 {
		start++
	}
	return start, start + code.code.KeywordOnlyCount()
}

// bindFunctionArguments applies positional and keyword values, defaults, and
// *args in CPython's conflict and missing-argument order.
func bindFunctionArguments(
	function *functionValue,
	arguments []Value,
	keywords *dictValue,
) ([]Value, *Exception) {
	code := function.code.code
	positionalCount := code.PositionalCount()
	positionalOnly := code.PositionalOnlyCount()
	keywordStart, keywordEnd := keywordOnlyRange(function.code)
	required := positionalCount - len(function.defaults)
	variadic := code.Flags()&bytecode.VarArgs != 0
	variadicKeywords := code.Flags()&bytecode.VarKeywords != 0
	locals := make([]Value, len(function.code.locals))
	var keywordArguments *dictValue
	if variadicKeywords {
		keywordArguments = &dictValue{}
		locals[keywordEnd] = keywordArguments
	}

	positionalGiven := min(len(arguments), positionalCount)
	copy(locals[:positionalGiven], arguments[:positionalGiven])
	if variadic {
		extra := make([]Value, len(arguments)-positionalGiven)
		copy(extra, arguments[positionalGiven:])
		locals[positionalCount] = &tupleValue{elements: extra}
	}

	if keywords != nil {
		keywordNames := make([]string, len(keywords.entries))
		var positionalOnlyNames []string
		for index, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException(
					"TypeError",
					code.QualifiedName()+"() keywords must be strings",
				)
			}
			keywordNames[index] = name.value
			if !variadicKeywords {
				for parameter := 0; parameter < positionalOnly; parameter++ {
					if function.code.locals[parameter] == name.value {
						positionalOnlyNames = append(positionalOnlyNames, name.value)
						break
					}
				}
			}
		}
		if len(positionalOnlyNames) != 0 {
			return nil, newException(
				"TypeError",
				code.QualifiedName()+
					"() got some positional-only arguments passed as keyword arguments: '"+
					strings.Join(positionalOnlyNames, ", ")+"'",
			)
		}
		for index, entry := range keywords.entries {
			name := keywordNames[index]
			parameterIndex := -1
			for parameter := positionalOnly; parameter < positionalCount; parameter++ {
				if function.code.locals[parameter] == name {
					parameterIndex = parameter
					break
				}
			}
			if parameterIndex < 0 {
				for parameter := keywordStart; parameter < keywordEnd; parameter++ {
					if function.code.locals[parameter] == name {
						parameterIndex = parameter
						break
					}
				}
			}
			if parameterIndex < 0 {
				if variadicKeywords {
					if exception := keywordArguments.set(entry.key, entry.value); exception != nil {
						return nil, exception
					}
					continue
				}
				return nil, newException(
					"TypeError",
					code.QualifiedName()+
						"() got an unexpected keyword argument '"+name+"'",
				)
			}
			if locals[parameterIndex] != nil {
				return nil, newException(
					"TypeError",
					code.QualifiedName()+"() got multiple values for argument '"+
						name+"'",
				)
			}
			locals[parameterIndex] = entry.value
		}
	}

	if len(arguments) > positionalCount && !variadic {
		return nil, tooManyPositionalError(
			function,
			len(arguments),
			positionalCount,
			required,
		)
	}
	var missing []string
	for parameter := 0; parameter < required; parameter++ {
		if locals[parameter] == nil {
			missing = append(missing, function.code.locals[parameter])
		}
	}
	if len(missing) != 0 {
		argument := "arguments"
		if len(missing) == 1 {
			argument = "argument"
		}
		return nil, newException(
			"TypeError",
			fmt.Sprintf(
				"%s() missing %d required positional %s: %s",
				code.QualifiedName(),
				len(missing),
				argument,
				formatMissingArguments(missing),
			),
		)
	}
	defaults := function.defaults
	if len(defaults) > positionalCount {
		defaults = defaults[len(defaults)-positionalCount:]
	}
	defaultStart := positionalCount - len(defaults)
	for parameter := defaultStart; parameter < positionalCount; parameter++ {
		if locals[parameter] == nil {
			locals[parameter] = defaults[parameter-defaultStart]
		}
	}
	var missingKeywordOnly []string
	for parameter := keywordStart; parameter < keywordEnd; parameter++ {
		if locals[parameter] != nil {
			continue
		}
		name := function.code.locals[parameter]
		if value, ok := function.keywordDefaults[name]; ok {
			locals[parameter] = value
			continue
		}
		missingKeywordOnly = append(missingKeywordOnly, name)
	}
	if len(missingKeywordOnly) != 0 {
		argument := "arguments"
		if len(missingKeywordOnly) == 1 {
			argument = "argument"
		}
		return nil, newException(
			"TypeError",
			fmt.Sprintf(
				"%s() missing %d required keyword-only %s: %s",
				code.QualifiedName(),
				len(missingKeywordOnly),
				argument,
				formatMissingArguments(missingKeywordOnly),
			),
		)
	}
	return locals, nil
}

func tooManyPositionalError(
	function *functionValue,
	actual int,
	expected int,
	required int,
) *Exception {
	signature := fmt.Sprintf("%d", expected)
	plural := expected != 1
	if len(function.defaults) != 0 {
		signature = fmt.Sprintf("from %d to %d", required, expected)
		plural = true
	}
	argument := "argument"
	if plural {
		argument = "arguments"
	}
	given := "were"
	if actual == 1 {
		given = "was"
	}
	return newException(
		"TypeError",
		fmt.Sprintf(
			"%s() takes %s positional %s but %d %s given",
			function.code.code.QualifiedName(),
			signature,
			argument,
			actual,
			given,
		),
	)
}

func formatMissingArguments(names []string) string {
	quoted := make([]string, len(names))
	for index, name := range names {
		quoted[index] = "'" + name + "'"
	}
	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	case 2:
		return quoted[0] + " and " + quoted[1]
	default:
		return strings.Join(quoted[:len(quoted)-1], ", ") +
			", and " + quoted[len(quoted)-1]
	}
}
