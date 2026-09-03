package runtime

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
	exceptionBase *exceptionTypeValue
	constructor   nativeTypeConstructor
	metaclass     *typeValue
	builtinBase   *builtinTypeValue
	enumKind      string
}

// memberDescriptorValue represents one storage name declared through
// __slots__. Values still live in the instance namespace, while the descriptor
// supplies the class-level metadata and data-descriptor protocol.
type memberDescriptorValue struct {
	name string
}

func (*memberDescriptorValue) TypeName() string { return "member_descriptor" }
func (descriptor *memberDescriptorValue) Repr() string {
	return "<member '" + descriptor.name + "'>"
}
func (*memberDescriptorValue) isValue() {}

func (*typeValue) TypeName() string { return "type" }
func (class *typeValue) Repr() string {
	if class.module == "" {
		return "<class '" + class.qualifiedName + "'>"
	}
	return "<class '" + class.module + "." + class.qualifiedName + "'>"
}
func (*typeValue) isValue() {}

func (class *typeValue) lookup(name string) (Value, bool) {
	for _, current := range class.methodResolutionOrder() {
		if value, found := current.namespace.get(name); found {
			return value, true
		}
	}
	if base := class.inheritedBuiltinBase(); base != nil {
		if value, found := builtinTypeMethod(base.name, name); found {
			return value, true
		}
	}
	return builtinTypeMethod("object", name)
}

func (class *typeValue) builtinExceptionBase() *exceptionTypeValue {
	for _, current := range class.methodResolutionOrder() {
		if current.exceptionBase != nil {
			return current.exceptionBase
		}
	}
	return nil
}

func (class *typeValue) isExceptionClass() bool {
	return class.builtinExceptionBase() != nil
}

func (class *typeValue) isSubclassOf(parent *typeValue) bool {
	for _, current := range class.methodResolutionOrder() {
		if current == parent {
			return true
		}
	}
	return false
}

func (class *typeValue) methodResolutionOrder() []*typeValue {
	if len(class.mro) != 0 {
		return class.mro
	}
	return []*typeValue{class}
}

func (class *typeValue) inheritedBuiltinBase() *builtinTypeValue {
	for _, current := range class.methodResolutionOrder() {
		if current.builtinBase != nil {
			return current.builtinBase
		}
	}
	return nil
}

type instanceValue struct {
	class      *typeValue
	attributes *Namespace
	sequence   *listValue
	tuple      *tupleValue
	mapping    *dictValue
}

func (instance *instanceValue) TypeName() string { return instance.class.name }
func (instance *instanceValue) Repr() string {
	if instance.sequence != nil {
		return instance.sequence.Repr()
	}
	if instance.tuple != nil {
		return instance.tuple.Repr()
	}
	if instance.mapping != nil {
		return instance.mapping.Repr()
	}
	name := instance.class.qualifiedName
	if instance.class.module != "" {
		name = instance.class.module + "." + name
	}
	return "<" + name + " object>"
}
func (*instanceValue) isValue() {}

type boundMethodValue struct {
	function *functionValue
	self     Value
}

type genericBoundMethodValue struct {
	callable Value
	self     Value
}

func (*genericBoundMethodValue) TypeName() string { return "method" }
func (method *genericBoundMethodValue) Repr() string {
	return "<bound method " + method.callable.Repr() + ">"
}
func (*genericBoundMethodValue) isValue() {}

func (method *genericBoundMethodValue) attribute(name string) (Value, bool) {
	switch name {
	case "__func__":
		return method.callable, true
	case "__self__":
		return method.self, true
	case "__call__":
		return method, true
	default:
		return directAttribute(method.callable, name)
	}
}

func (*boundMethodValue) TypeName() string { return "method" }
func (method *boundMethodValue) Repr() string {
	return "<bound method " + method.function.code.code.QualifiedName() + ">"
}
func (*boundMethodValue) isValue() {}

func (method *boundMethodValue) attribute(name string) (Value, bool) {
	switch name {
	case "__func__":
		return method.function, true
	case "__self__":
		return method.self, true
	default:
		return method.function.attribute(name)
	}
}

type boundNativeMethodValue struct {
	function *nativeFunctionValue
	self     Value
}

func (*boundNativeMethodValue) TypeName() string { return "builtin_function_or_method" }
func (method *boundNativeMethodValue) Repr() string {
	return "<built-in method " + method.function.name + ">"
}
func (*boundNativeMethodValue) isValue() {}

func (method *boundNativeMethodValue) attribute(name string) (Value, bool) {
	switch name {
	case "__self__":
		return method.self, true
	default:
		return method.function.attribute(name)
	}
}

type classBuild struct {
	name          string
	qualifiedName string
	module        string
	namespace     *Namespace
	bases         []*typeValue
	exceptionBase *exceptionTypeValue
	metaclass     *typeValue
	builtinBase   *builtinTypeValue
}

func (build *classBuild) finish(bodyResult Value) Value {
	class := &typeValue{
		name:          build.name,
		qualifiedName: build.qualifiedName,
		module:        build.module,
		namespace:     build.namespace,
		bases:         build.bases,
		exceptionBase: build.exceptionBase,
		metaclass:     build.metaclass,
		builtinBase:   build.builtinBase,
	}
	class.mro, _ = calculateMRO(class, build.bases)
	initializeClassSlots(class)
	initializeEnumSubclass(class)
	initializeTestCaseSubclassState(class)
	if classCell, ok := bodyResult.(*cellValue); ok {
		classCell.value = class
	}
	return class
}

// initializeClassSlots validates declared slot names and installs native member
// descriptors for storage owned by each instance.
func initializeClassSlots(class *typeValue) {
	value, found := class.namespace.get("__slots__")
	if !found {
		return
	}
	var slots []Value
	switch value := value.(type) {
	case *stringValue:
		slots = []Value{value}
	case *listValue:
		slots = value.elements
	case *tupleValue:
		slots = value.elements
	default:
		return
	}
	for _, slot := range slots {
		name, ok := slot.(*stringValue)
		if !ok || name.value == "__dict__" || name.value == "__weakref__" {
			continue
		}
		if _, exists := class.namespace.get(name.value); !exists {
			class.namespace.values[name.value] = &memberDescriptorValue{name: name.value}
		}
	}
}

func initializeTestCaseSubclassState(class *typeValue) {
	_, definesCleanup := class.namespace.get("doClassCleanups")
	inheritsCleanup := false
	for _, base := range class.bases {
		if _, found := base.lookup("doClassCleanups"); found {
			inheritsCleanup = true
			break
		}
	}
	if !definesCleanup && !inheritsCleanup {
		return
	}
	class.namespace.values["_classSetupFailed"] = falseSingleton
	class.namespace.values["_class_cleanups"] = &listValue{}
	class.namespace.values["tearDown_exceptions"] = &listValue{}
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
	metaclass, keywordException := classMetaclass(keywords)
	if keywordException != nil {
		return instructionOutcome{kind: raised, exception: keywordException}, nil
	}
	var bases []*typeValue
	var exceptionBase *exceptionTypeValue
	var builtinBase *builtinTypeValue
	for _, argument := range arguments[2:] {
		if alias, ok := argument.(*genericAliasValue); ok {
			argument = alias.origin
		}
		switch classBase := argument.(type) {
		case *typeValue:
			bases = append(bases, classBase)
			if metaclass == nil {
				metaclass = classBase.metaclass
			}
		case *exceptionTypeValue:
			if exceptionBase == nil {
				exceptionBase = classBase
			} else {
				exceptionBase = &exceptionTypeValue{
					name: name.value, base: exceptionBase, additionalBase: classBase,
				}
			}
		case *builtinTypeValue:
			if classBase.name != "type" && builtinBase != nil {
				return instructionOutcome{
					kind:      raised,
					exception: newException("TypeError", "multiple built-in bases are not supported"),
				}, nil
			}
			if classBase.name != "type" {
				builtinBase = classBase
			}
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
			metaclass:     metaclass,
			builtinBase:   builtinBase,
		},
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

// classMetaclass validates the currently supported metaclass-only keyword map
// and returns the Python metaclass recorded on the new class.
func classMetaclass(keywords *dictValue) (*typeValue, *Exception) {
	if keywords == nil || len(keywords.entries) == 0 {
		return nil, nil
	}
	if len(keywords.entries) != 1 {
		return nil, newException("TypeError", "unsupported class keyword argument")
	}
	entry := keywords.entries[0]
	name, ok := entry.key.(*stringValue)
	if !ok || name.value != "metaclass" {
		return nil, newException("TypeError", "unsupported class keyword argument")
	}
	metaclass, ok := entry.value.(*typeValue)
	if !ok {
		return nil, newException("TypeError", "metaclass must be a type")
	}
	return metaclass, nil
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
	for _, current := range class.methodResolutionOrder() {
		if current.constructor != nil {
			return executeNativeTypeCall(
				caller,
				instruction,
				base,
				class,
				current.constructor,
				arguments,
				keywords,
			)
		}
	}
	if allocator, found := classDefinedNew(class); found {
		newArguments := make([]Value, len(arguments)+1)
		newArguments[0] = class
		copy(newArguments[1:], arguments)
		value, exception, err := callValueSynchronouslyWithKeywords(
			caller, allocator, newArguments, keywords,
		)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		instance, initializes := value.(*instanceValue)
		if initializes && instance.class.isSubclassOf(class) {
			initializer, found := class.lookup("__init__")
			if native, ok := initializer.(*nativeFunctionValue); ok && native.name == "object.__init__" {
				found = false
			}
			if found {
				result, exception, err := callValueSynchronouslyWithKeywords(
					caller, bindCallable(initializer, instance), arguments, keywords,
				)
				if err != nil {
					return instructionOutcome{}, err
				}
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				if result != None {
					return instructionOutcome{kind: raised, exception: newException(
						"TypeError", "__init__() should return None, not '"+result.TypeName()+"'",
					)}, nil
				}
			}
		}
		for index := base; index < len(caller.stack); index++ {
			caller.stack[index] = nil
		}
		caller.stack = caller.stack[:base]
		return pushOutcome(caller, instruction, value)
	}
	instance := &instanceValue{class: class, attributes: newNamespace()}
	if class.builtinBase == builtinTypeNamed("list") {
		instance.sequence = &listValue{}
	}
	if class.builtinBase == builtinTypeNamed("dict") {
		instance.mapping = &dictValue{}
	}
	if class.builtinBase != nil && class.builtinBase.name == "module" && len(arguments) >= 1 {
		if name, ok := arguments[0].(*stringValue); ok {
			instance.attributes.values["__name__"] = name
			arguments = arguments[1:]
		}
	}
	initializerValue, hasInitializer := class.lookup("__init__")
	if initializer, ok := initializerValue.(*nativeFunctionValue); ok && initializer.name == "object.__init__" {
		hasInitializer = false
	}
	if !hasInitializer {
		if class.builtinBase == builtinTypeNamed("list") &&
			(keywords == nil || len(keywords.entries) == 0) {
			value, exception, err := constructBuiltinCollection(caller.runtime, "list", arguments)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			instance.sequence = value.(*listValue)
			arguments = nil
		}
		if class.builtinBase == builtinTypeNamed("dict") &&
			(keywords == nil || len(keywords.entries) == 0) {
			value, exception, err := constructBuiltinDict(caller.runtime, arguments)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			instance.mapping = value.(*dictValue)
			arguments = nil
		}
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
	if err == nil && outcome.kind == called {
		outcome.frame.instanceInit = &instanceInit{
			instance:    instance,
			instruction: instruction,
		}
	}
	return outcome, err
}

// classDefinedNew returns an allocator explicitly supplied by a Python class
// or one of its Python bases. Built-in allocation remains on the constructor
// paths above.
func classDefinedNew(class *typeValue) (Value, bool) {
	for _, current := range class.methodResolutionOrder() {
		if allocator, found := current.namespace.get("__new__"); found {
			return allocator, true
		}
	}
	return nil, false
}

// calculateMRO applies the C3 merge to each base order plus the direct base
// list, returning false when no monotonic linearization exists.
func calculateMRO(class *typeValue, bases []*typeValue) ([]*typeValue, bool) {
	sequences := make([][]*typeValue, 0, len(bases)+1)
	for _, base := range bases {
		sequences = append(sequences, append([]*typeValue(nil), base.methodResolutionOrder()...))
	}
	sequences = append(sequences, append([]*typeValue(nil), bases...))
	result := []*typeValue{class}
	for {
		sequences = discardEmptyMROSequences(sequences)
		if len(sequences) == 0 {
			return result, true
		}
		candidate, found := nextMROCandidate(sequences)
		if !found {
			return nil, false
		}
		result = append(result, candidate)
		for index := range sequences {
			if len(sequences[index]) != 0 && sequences[index][0] == candidate {
				sequences[index] = sequences[index][1:]
			}
		}
	}
}

func discardEmptyMROSequences(sequences [][]*typeValue) [][]*typeValue {
	nonempty := sequences[:0]
	for _, sequence := range sequences {
		if len(sequence) != 0 {
			nonempty = append(nonempty, sequence)
		}
	}
	return nonempty
}

// nextMROCandidate selects the first sequence head absent from every other
// sequence tail, which is the next valid C3 linearization entry.
func nextMROCandidate(sequences [][]*typeValue) (*typeValue, bool) {
	for _, sequence := range sequences {
		candidate := sequence[0]
		valid := true
		for _, other := range sequences {
			for _, tail := range other[1:] {
				if tail == candidate {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
		}
		if valid {
			return candidate, true
		}
	}
	return nil, false
}

var _ Value = (*buildClassValue)(nil)
var _ Value = (*typeValue)(nil)
var _ Value = (*instanceValue)(nil)
var _ Value = (*boundMethodValue)(nil)
var _ Value = (*boundNativeMethodValue)(nil)
