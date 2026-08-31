package runtime

import (
	"fmt"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type functionValue struct {
	code            *preparedCode
	globals         *Namespace
	defaults        []Value
	keywordDefaults map[string]Value
	closure         []*cellValue
	annotate        *functionValue
	annotations     *dictValue
	typeParams      *tupleValue
}

func (*functionValue) TypeName() string { return "function" }
func (function *functionValue) Repr() string {
	return "<function " + function.code.code.QualifiedName() + ">"
}
func (*functionValue) isValue() {}

// executeSetFunctionTypeParameters attaches compiler-created PEP 695
// parameters and keeps the function on the operand stack.
func executeSetFunctionTypeParameters(
	frame *frame,
	instruction int,
) (instructionOutcome, error) {
	targetValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	parametersValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	function, ok := targetValue.(*functionValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"function type parameter target is not a function",
		)
	}
	parameters, ok := parametersValue.(*tupleValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"function type parameters payload is not a tuple",
		)
	}
	for _, parameter := range parameters.elements {
		if !isTypeParameter(parameter) {
			return instructionOutcome{}, frame.failure(
				instruction,
				"function type parameter payload contains a non-type-parameter value",
			)
		}
	}
	function.typeParams = parameters
	return pushOutcome(frame, instruction, function)
}

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
	case *builtinFunctionValue:
		return executeBuiltinFunctionCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *propertyAccessorMethod:
		return executePropertyAccessorCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *listAppendMethod:
		return executeListAppendCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *listPopMethod:
		return executeListPopCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *listExtendMethod:
		return executeListExtendCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *dictionaryPopMethod:
		return executeDictionaryPopCall(
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
	case *nativeTypeValue:
		return executeNativeTypeCall(
			caller,
			instruction,
			base,
			callable,
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
	case *asyncGeneratorAIterMethod:
		return executeAsyncGeneratorAIterCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *asyncGeneratorANextMethod:
		return executeAsyncGeneratorANextCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *asyncGeneratorASendMethod:
		return executeAsyncGeneratorASendCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *asyncGeneratorAThrowMethod:
		return executeAsyncGeneratorAThrowCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *asyncGeneratorACloseMethod:
		return executeAsyncGeneratorACloseCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *generatorSendMethod:
		return executeGeneratorSendCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *generatorThrowMethod:
		return executeGeneratorThrowCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *generatorCloseMethod:
		return executeGeneratorCloseCall(
			caller,
			instruction,
			base,
			callable,
			arguments,
			keywords,
		)
	case *staticMethodValue:
		return executeFunctionCall(
			caller,
			instruction,
			base,
			callable.callable,
			arguments,
			keywords,
		)
	case *instanceValue:
		method, found := lookupInstanceSpecial(callable, "__call__")
		if found {
			return executeFunctionCall(
				caller,
				instruction,
				base,
				method,
				arguments,
				keywords,
			)
		}
	case *boundMethodValue:
		boundArguments := make([]Value, len(arguments)+1)
		boundArguments[0] = callable.self
		copy(boundArguments[1:], arguments)
		return executeFunctionCall(
			caller,
			instruction,
			base,
			callable.callable,
			boundArguments,
			keywords,
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
	child, exception, err := newFunctionFrame(
		caller,
		instruction,
		function,
		arguments,
		keywords,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	flags := function.code.code.Flags()
	suspendedFlags := bytecode.Generator | bytecode.Coroutine | bytecode.AsyncGenerator
	if flags&suspendedFlags != 0 {
		kind := generatorObject
		switch {
		case flags&bytecode.Coroutine != 0:
			kind = coroutineObject
		case flags&bytecode.AsyncGenerator != 0:
			kind = asyncGeneratorObject
		}
		generator := &generatorValue{
			frame:         child,
			qualifiedName: function.code.code.QualifiedName(),
			kind:          kind,
			state:         generatorCreated,
		}
		child.previous = nil
		child.generator = generator
		return pushOutcome(caller, instruction, generator)
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

// isCallableValue mirrors the concrete values accepted by executeFunctionCall
// and treats any class-level __call__ entry as an instance call slot.
func isCallableValue(value Value) bool {
	switch value := value.(type) {
	case *instanceValue:
		_, found := value.class.lookup("__call__")
		return found
	case *builtinFunctionValue,
		*nativeTypeValue,
		*propertyAccessorMethod,
		*listAppendMethod,
		*listPopMethod,
		*listExtendMethod,
		*dictionaryPopMethod,
		*buildClassValue,
		*typeValue,
		*exceptionTypeValue,
		*asyncGeneratorAIterMethod,
		*asyncGeneratorANextMethod,
		*asyncGeneratorASendMethod,
		*asyncGeneratorAThrowMethod,
		*asyncGeneratorACloseMethod,
		*generatorSendMethod,
		*generatorThrowMethod,
		*generatorCloseMethod,
		*staticMethodValue,
		*boundMethodValue,
		*functionValue:
		return true
	default:
		return false
	}
}

func newFunctionFrame(
	caller *frame,
	instruction int,
	function *functionValue,
	arguments []Value,
	keywords *dictValue,
) (*frame, *Exception, error) {
	locals, exception := bindFunctionArguments(function, arguments, keywords)
	if exception != nil {
		return nil, exception, nil
	}
	deref, ok := initializeDeref(function.code, locals, function.closure)
	if !ok {
		return nil, nil, caller.failure(
			instruction,
			fmt.Sprintf(
				"function closure has %d cells for %d free variables",
				len(function.closure),
				len(function.code.freeVars),
			),
		)
	}
	return &frame{
		runtime:    caller.runtime,
		code:       function.code,
		stack:      make([]Value, 0, function.code.stackSize),
		fastLocals: locals,
		deref:      deref,
		locals:     newNamespace(),
		globals:    function.globals,
		builtins:   caller.builtins,
		previous:   caller,
	}, nil, nil
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
	defaultStart := positionalCount - len(function.defaults)
	for parameter := defaultStart; parameter < positionalCount; parameter++ {
		if locals[parameter] == nil {
			locals[parameter] = function.defaults[parameter-defaultStart]
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
