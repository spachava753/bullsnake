package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

type buildClassValue struct{}

func (*buildClassValue) TypeName() string { return "builtin_function_or_method" }
func (*buildClassValue) Repr() string     { return "<built-in function __build_class__>" }
func (*buildClassValue) isValue()         {}

var buildClassSingleton = &buildClassValue{}

type typeValue struct {
	name          string
	qualifiedName string
	module        string
	namespace     *Namespace
	bases         []*typeValue
	exceptionBase *exceptionTypeValue
}

func (*typeValue) TypeName() string { return "type" }
func (class *typeValue) Repr() string {
	if class.module == "" {
		return "<class '" + class.qualifiedName + "'>"
	}
	return "<class '" + class.module + "." + class.qualifiedName + "'>"
}
func (*typeValue) isValue() {}

func (class *typeValue) lookup(name string) (Value, bool) {
	for current := class; current != nil; {
		if value, found := current.namespace.get(name); found {
			return value, true
		}
		if len(current.bases) == 0 {
			break
		}
		current = current.bases[0]
	}
	return nil, false
}

func (class *typeValue) builtinExceptionBase() *exceptionTypeValue {
	for current := class; current != nil; {
		if current.exceptionBase != nil {
			return current.exceptionBase
		}
		if len(current.bases) == 0 {
			return nil
		}
		current = current.bases[0]
	}
	return nil
}

func (class *typeValue) isExceptionClass() bool {
	return class.builtinExceptionBase() != nil
}

func (class *typeValue) isSubclassOf(parent *typeValue) bool {
	for current := class; current != nil; {
		if current == parent {
			return true
		}
		if len(current.bases) == 0 {
			return false
		}
		current = current.bases[0]
	}
	return false
}

type instanceValue struct {
	class      *typeValue
	attributes *Namespace
}

func (instance *instanceValue) TypeName() string { return instance.class.name }
func (instance *instanceValue) Repr() string {
	name := instance.class.qualifiedName
	if instance.class.module != "" {
		name = instance.class.module + "." + name
	}
	return "<" + name + " object>"
}
func (*instanceValue) isValue() {}

type boundMethodValue struct {
	function *functionValue
	self     *instanceValue
}

func (*boundMethodValue) TypeName() string { return "method" }
func (method *boundMethodValue) Repr() string {
	return "<bound method " + method.function.code.code.QualifiedName() + ">"
}
func (*boundMethodValue) isValue() {}

type classBuild struct {
	name          string
	qualifiedName string
	module        string
	namespace     *Namespace
	bases         []*typeValue
	exceptionBase *exceptionTypeValue
}

func (build *classBuild) finish(bodyResult Value) Value {
	class := &typeValue{
		name:          build.name,
		qualifiedName: build.qualifiedName,
		module:        build.module,
		namespace:     build.namespace,
		bases:         build.bases,
		exceptionBase: build.exceptionBase,
	}
	if classCell, ok := bodyResult.(*cellValue); ok {
		classCell.value = class
	}
	return class
}

// executeBuildClassCall starts one no-base class body with its own local
// namespace. The dispatch loop finishes type creation when this frame returns.
func executeBuildClassCall(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if len(arguments) < 2 {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "__build_class__: not enough arguments"),
		}, nil
	}
	body, ok := arguments[0].(*functionValue)
	if !ok {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "__build_class__: func must be a function"),
		}, nil
	}
	name, ok := arguments[1].(*stringValue)
	if !ok {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "__build_class__: name is not a string"),
		}, nil
	}
	if keywords != nil && len(keywords.entries) != 0 {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "class keyword arguments are not supported"),
		}, nil
	}
	if len(arguments) > 3 {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "multiple inheritance is not supported"),
		}, nil
	}
	var bases []*typeValue
	var exceptionBase *exceptionTypeValue
	if len(arguments) == 3 {
		switch classBase := arguments[2].(type) {
		case *typeValue:
			bases = []*typeValue{classBase}
		case *exceptionTypeValue:
			exceptionBase = classBase
		default:
			return instructionOutcome{
				kind:      raised,
				exception: newException("TypeError", "class base is not a type"),
			}, nil
		}
	}
	locals, exception := bindFunctionArguments(body, nil, nil)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	deref, initialized := initializeDeref(body.code, locals, body.closure)
	if !initialized {
		return instructionOutcome{}, caller.failure(
			instruction,
			"class body closure does not match its free variables",
		)
	}
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	namespace := newNamespace()
	module := ""
	if value, found := body.globals.get("__name__"); found {
		if moduleName, isString := value.(*stringValue); isString {
			module = moduleName.value
		}
	}
	child := &frame{
		runtime:    caller.runtime,
		code:       body.code,
		stack:      make([]Value, 0, body.code.stackSize),
		fastLocals: locals,
		deref:      deref,
		locals:     namespace,
		globals:    body.globals,
		builtins:   caller.builtins,
		previous:   caller,
		classBuild: &classBuild{
			name:          name.value,
			qualifiedName: body.code.code.QualifiedName(),
			module:        module,
			namespace:     namespace,
			bases:         bases,
			exceptionBase: exceptionBase,
		},
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

type instanceInit struct {
	instance    *instanceValue
	instruction int
}

// executeTypeCall allocates an instance, either completes an empty constructor
// immediately or invokes a plain __init__, and marks its frame to return the
// instance only after the initializer returns None.
func executeTypeCall(
	caller *frame,
	instruction int,
	base int,
	class *typeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if class.isExceptionClass() {
		return executeUserExceptionTypeCall(
			caller,
			instruction,
			base,
			class,
			arguments,
			keywords,
		)
	}
	instance := &instanceValue{class: class, attributes: newNamespace()}
	initializerValue, hasInitializer := class.lookup("__init__")
	if !hasInitializer {
		if len(arguments) != 0 || (keywords != nil && len(keywords.entries) != 0) {
			return instructionOutcome{
				kind:      raised,
				exception: newException("TypeError", class.name+"() takes no arguments"),
			}, nil
		}
		for index := base; index < len(caller.stack); index++ {
			caller.stack[index] = nil
		}
		caller.stack = caller.stack[:base]
		return pushOutcome(caller, instruction, instance)
	}
	initializer, callable := initializerValue.(*functionValue)
	if !callable {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+initializerValue.TypeName()+"' object is not callable",
			),
		}, nil
	}
	bound := &boundMethodValue{function: initializer, self: instance}
	outcome, err := executeFunctionCall(
		caller,
		instruction,
		base,
		bound,
		arguments,
		keywords,
	)
	suspendedFlags := bytecode.Generator | bytecode.Coroutine | bytecode.AsyncGenerator
	if err == nil && outcome.kind == advance &&
		initializer.code.code.Flags()&suspendedFlags != 0 {
		result, ok := caller.pop()
		if !ok {
			return instructionOutcome{}, caller.failure(
				instruction,
				"suspended initializer produced no result",
			)
		}
		generator, ok := result.(*generatorValue)
		if !ok {
			return instructionOutcome{}, caller.failure(
				instruction,
				"suspended initializer result has the wrong type",
			)
		}
		resultType := generator.TypeName()
		generator.complete()
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"__init__() should return None, not '"+resultType+"'",
			),
		}, nil
	}
	if err == nil && outcome.kind == called {
		outcome.frame.instanceInit = &instanceInit{
			instance:    instance,
			instruction: instruction,
		}
	}
	return outcome, err
}

var _ Value = (*buildClassValue)(nil)
var _ Value = (*typeValue)(nil)
var _ Value = (*instanceValue)(nil)
var _ Value = (*boundMethodValue)(nil)
