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
	mro           []*typeValue
	objectBase    bool
	abstract      bool
	nativeBase    *nativeTypeValue
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

func typeTuple(classes []*typeValue) *tupleValue {
	elements := make([]Value, len(classes))
	for index, class := range classes {
		elements[index] = class
	}
	return &tupleValue{elements: elements}
}

func readOnlyTypeMetadata(name string) bool {
	return name == "__mro__" || name == "__base__" || name == "__bases__"
}

func (class *typeValue) lookup(name string) (Value, bool) {
	for _, current := range class.mro {
		if value, found := current.namespace.get(name); found {
			return value, true
		}
	}
	return nil, false
}

func (class *typeValue) builtinExceptionBase() *exceptionTypeValue {
	for _, current := range class.mro {
		if current.exceptionBase != nil {
			return current.exceptionBase
		}
	}
	return nil
}

func (class *typeValue) builtinExceptionBaseFor(
	parent *exceptionTypeValue,
) *exceptionTypeValue {
	for _, current := range class.mro {
		if current.exceptionBase != nil && current.exceptionBase.isSubclassOf(parent) {
			return current.exceptionBase
		}
	}
	return nil
}

func (class *typeValue) isSubclassOfBuiltinException(parent *exceptionTypeValue) bool {
	return class.builtinExceptionBaseFor(parent) != nil
}

func (class *typeValue) isExceptionClass() bool {
	return class.builtinExceptionBase() != nil
}

func (class *typeValue) isSubclassOf(parent *typeValue) bool {
	for _, current := range class.mro {
		if current == parent {
			return true
		}
	}
	return false
}

func (class *typeValue) nativeClassBase() *nativeTypeValue {
	for _, current := range class.mro {
		if current.nativeBase != nil {
			return current.nativeBase
		}
	}
	return nil
}

func (class *typeValue) isSubclassOfNative(parent *nativeTypeValue) bool {
	if parent == objectNativeType {
		return true
	}
	base := class.nativeClassBase()
	return base != nil && base.isSubclassOf(parent)
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
	callable Value
	self     Value
}

func (*boundMethodValue) TypeName() string { return "method" }
func (method *boundMethodValue) Repr() string {
	name := method.callable.Repr()
	if function, ok := method.callable.(*functionValue); ok {
		name = function.code.code.QualifiedName()
	}
	return "<bound method " + name + ">"
}
func (*boundMethodValue) isValue() {}

func lookupInstanceSpecial(instance *instanceValue, name string) (Value, bool) {
	value, found := instance.class.lookup(name)
	if !found {
		return nil, false
	}
	if bound, descriptor := bindMethodDescriptor(value, instance.class); descriptor {
		return bound, true
	}
	if function, bind := value.(*functionValue); bind {
		value = &boundMethodValue{callable: function, self: instance}
	}
	return value, true
}

type classBuild struct {
	name              string
	qualifiedName     string
	module            string
	instruction       int
	namespace         *Namespace
	bases             []*typeValue
	objectBase        bool
	nativeBase        *nativeTypeValue
	exceptionBase     *exceptionTypeValue
	namespaceOrder    []string
	namespacePosition map[string]int
}

func (build *classBuild) recordStore(name string) {
	if _, found := build.namespacePosition[name]; found {
		return
	}
	build.namespacePosition[name] = len(build.namespaceOrder)
	build.namespaceOrder = append(build.namespaceOrder, name)
}

// finish computes the class's C3 order before publishing native property names
// and filling the compiler-created __class__ cell.
func (build *classBuild) finish(bodyResult Value) (Value, *Exception) {
	class := &typeValue{
		name:          build.name,
		qualifiedName: build.qualifiedName,
		module:        build.module,
		namespace:     build.namespace,
		bases:         build.bases,
		objectBase:    build.objectBase,
		nativeBase:    build.nativeBase,
		exceptionBase: build.exceptionBase,
	}
	mro, exception := calculateMRO(class, build.bases)
	if exception != nil {
		return nil, exception
	}
	class.mro = mro
	for index, name := range build.namespaceOrder {
		if build.namespacePosition[name] != index {
			continue
		}
		if property, ok := build.namespace.values[name].(*propertyValue); ok {
			property.name = name
		}
	}
	if classCell, ok := bodyResult.(*cellValue); ok {
		classCell.value = class
	}
	return class, nil
}

// resolveClassBases separates user, exception, object, and the supported sole
// native descriptor bases while retaining the mixed-native rejection boundary.
func resolveClassBases(
	baseValues []Value,
) ([]*typeValue, *exceptionTypeValue, *nativeTypeValue, bool, *Exception) {
	var bases []*typeValue
	var exceptionBase *exceptionTypeValue
	var nativeBase *nativeTypeValue
	objectBase := len(baseValues) == 0
	for _, baseValue := range baseValues {
		switch classBase := baseValue.(type) {
		case *typeValue:
			if classBase.nativeClassBase() != nil && len(baseValues) != 1 {
				return nil, nil, nil, false, newException(
					"TypeError",
					"multiple inheritance with native bases is not supported",
				)
			}
			bases = append(bases, classBase)
		case *exceptionTypeValue:
			if len(baseValues) != 1 {
				return nil, nil, nil, false, newException(
					"TypeError",
					"multiple inheritance with built-in exception bases is not supported",
				)
			}
			exceptionBase = classBase
		case *nativeTypeValue:
			if len(baseValues) != 1 {
				return nil, nil, nil, false, newException(
					"TypeError",
					"multiple inheritance with native bases is not supported",
				)
			}
			switch classBase {
			case objectNativeType:
				objectBase = true
			case classMethodNativeType, staticMethodNativeType, propertyNativeType:
				nativeBase = classBase
			default:
				return nil, nil, nil, false, newException(
					"TypeError",
					"native base '"+classBase.name+"' is not supported",
				)
			}
		default:
			return nil, nil, nil, false, newException(
				"TypeError",
				"class base is not a type",
			)
		}
	}
	return bases, exceptionBase, nativeBase, objectBase, nil
}

// executeBuildClassCall starts one class body with its own local namespace. The
// dispatch loop computes its MRO and finishes type creation when that frame returns.
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
	baseValues := arguments[2:]
	bases, exceptionBase, nativeBase, objectBase, baseException := resolveClassBases(
		baseValues,
	)
	if baseException != nil {
		return instructionOutcome{kind: raised, exception: baseException}, nil
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
			name:              name.value,
			qualifiedName:     body.code.code.QualifiedName(),
			module:            module,
			instruction:       instruction,
			namespace:         namespace,
			bases:             bases,
			objectBase:        objectBase,
			nativeBase:        nativeBase,
			exceptionBase:     exceptionBase,
			namespacePosition: make(map[string]int),
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
	if class.nativeClassBase() != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"NotImplementedError",
			"native descriptor subclasses cannot be instantiated",
		)), nil
	}
	initializerValue, hasInitializer := class.lookup("__init__")
	if !hasInitializer && (len(arguments) != 0 || (keywords != nil && len(keywords.entries) != 0)) {
		return raiseOutcome(newException("TypeError", class.name+"() takes no arguments")), nil
	}
	if class.abstract {
		discardCallSegment(caller, base)
		methods, found := class.namespace.get("__abstractmethods__")
		if !found {
			return raiseOutcome(newException("AttributeError", "__abstractmethods__")), nil
		}
		return startCollectionConstructor(caller, &collectionConstructorCall{
			instruction: instruction,
			kind:        collectionSorted,
			iterable:    methods,
			sorting: &sortCall{
				instruction:   instruction,
				key:           None,
				reverseValue:  falseSingleton,
				abstractClass: class,
			},
		})
	}
	instance := &instanceValue{class: class, attributes: newNamespace()}
	if !hasInitializer {
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
	bound := &boundMethodValue{callable: initializer, self: instance}
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
