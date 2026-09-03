package runtime

import (
	"hash/fnv"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strings"
)

// builtinPrint writes through the interpreter-local sys.stdout value, so
// unittest buffering and embedder-provided streams observe the same calls.
func (runtimeState *Runtime) builtinPrint(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	separator, ending := " ", "\n"
	var destination Value
	flush := false
	if sys, found := runtimeState.modules["sys"]; found {
		destination, _ = sys.globals.get("stdout")
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "print() keywords must be strings"), nil
			}
			switch name.value {
			case "sep":
				if entry.value != None {
					text, stringOK := entry.value.(*stringValue)
					if !stringOK {
						return nil, newException("TypeError", "sep must be None or a string"), nil
					}
					separator = text.value
				}
			case "end":
				if entry.value != None {
					text, stringOK := entry.value.(*stringValue)
					if !stringOK {
						return nil, newException("TypeError", "end must be None or a string"), nil
					}
					ending = text.value
				}
			case "file":
				if entry.value != None {
					destination = entry.value
				}
			case "flush":
				flush = truthValue(entry.value)
			default:
				return nil, newException(
					"TypeError", "print() got an unexpected keyword argument '"+name.value+"'",
				), nil
			}
		}
	}
	if destination == nil || destination == None {
		return nil, newException("RuntimeError", "lost sys.stdout"), nil
	}
	parts := make([]string, len(arguments))
	for index, argument := range arguments {
		value, exception, err := pythonString(caller, argument)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		parts[index] = value.value
	}
	writer, found := destination.(attributeValue)
	if !found {
		return nil, newException("AttributeError", "print destination has no write method"), nil
	}
	write, found := writer.attribute("write")
	if !found {
		return nil, newException("AttributeError", "print destination has no write method"), nil
	}
	_, exception, err := callValueSynchronously(
		caller, write, []Value{&stringValue{value: strings.Join(parts, separator) + ending}},
	)
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if flush {
		flushCall, found := writer.attribute("flush")
		if !found {
			return nil, newException("AttributeError", "print destination has no flush method"), nil
		}
		_, exception, err = callValueSynchronously(caller, flushCall, nil)
		if err != nil || exception != nil {
			return nil, exception, err
		}
	}
	return None, nil, nil
}

func builtinGlobals(caller *frame, _ []Value) (Value, *Exception, error) {
	return &namespaceValue{namespace: caller.globals}, nil, nil
}

func builtinLocals(caller *frame, _ []Value) (Value, *Exception, error) {
	return &namespaceValue{namespace: caller.locals}, nil, nil
}

// builtinGetAttr resolves concrete metadata first, then invokes a user
// __getattr__ fallback and applies the optional default only to missing values.
func builtinGetAttr(caller *frame, arguments []Value) (Value, *Exception, error) {
	name, ok := arguments[1].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "attribute name must be string"), nil
	}
	if name.value == "__annotations__" {
		switch owner := arguments[0].(type) {
		case *typeValue:
			return materializeClassAnnotations(caller, owner)
		}
	}
	if value, found := directAttribute(arguments[0], name.value); found {
		if instance, ok := arguments[0].(*instanceValue); ok {
			if resolved, descriptor, exception, err := resolvePythonDescriptor(
				caller, value, instance, instance.class,
			); descriptor {
				if exception != nil && exception.class == attributeErrorType && len(arguments) == 3 {
					return arguments[2], nil, nil
				}
				return resolved, exception, err
			}
		}
		if class, ok := arguments[0].(*typeValue); ok {
			if resolved, descriptor, exception, err := resolvePythonDescriptor(
				caller, value, None, class,
			); descriptor {
				if exception != nil && exception.class == attributeErrorType && len(arguments) == 3 {
					return arguments[2], nil, nil
				}
				return resolved, exception, err
			}
		}
		if descriptor, ok := value.(*descriptorValue); ok && descriptor.kind == propertyDescriptor {
			// Access through a class returns the property object itself; only an
			// instance access invokes its getter.
			if _, classAccess := arguments[0].(*typeValue); classAccess {
				return descriptor, nil, nil
			}
			value, exception, err := callValueSynchronously(
				caller, descriptor.callable, []Value{arguments[0]},
			)
			if exception != nil && exception.class == attributeErrorType && len(arguments) == 3 {
				return arguments[2], nil, nil
			}
			return value, exception, err
		}
		return value, nil, nil
	}
	if instance, ok := arguments[0].(*instanceValue); ok {
		if fallback, found := instance.class.lookup("__getattr__"); found {
			value, exception, err := callValueSynchronously(
				caller, bindCallable(fallback, instance), []Value{name},
			)
			if err != nil || exception != nil {
				if exception != nil && exception.class == attributeErrorType && len(arguments) == 3 {
					return arguments[2], nil, nil
				}
				return nil, exception, err
			}
			return value, nil, nil
		}
	}
	if module, ok := arguments[0].(*Module); ok {
		if fallback, found := module.globals.get("__getattr__"); found {
			value, exception, err := callValueSynchronously(caller, fallback, []Value{name})
			if err != nil || exception != nil {
				if exception != nil && exception.class == attributeErrorType && len(arguments) == 3 {
					return arguments[2], nil, nil
				}
				return nil, exception, err
			}
			return value, nil, nil
		}
	}
	if len(arguments) == 3 {
		return arguments[2], nil, nil
	}
	return nil, newException("AttributeError", missingAttributeMessage(arguments[0], name.value)), nil
}

func missingAttributeMessage(owner Value, name string) string {
	switch owner := owner.(type) {
	case *Module:
		return "module '" + owner.name + "' has no attribute '" + name + "'"
	case *typeValue:
		return "type object '" + owner.name + "' has no attribute '" + name + "'"
	default:
		return "'" + owner.TypeName() + "' object has no attribute '" + name + "'"
	}
}

// builtinHasAttr converts only AttributeError from the shared getter into false.
func builtinHasAttr(caller *frame, arguments []Value) (Value, *Exception, error) {
	_, ok := arguments[1].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "attribute name must be string"), nil
	}
	_, exception, err := builtinGetAttr(caller, arguments)
	if exception != nil && exception.class == attributeErrorType {
		return falseSingleton, nil, nil
	}
	if err != nil || exception != nil {
		return nil, exception, err
	}
	return trueSingleton, nil, nil
}

// directAttribute performs synchronous attribute lookup outside the VM opcode path.
func directAttribute(owner Value, name string) (Value, bool) {
	if name == "__class__" {
		instance, isInstance := owner.(*instanceValue)
		attribute, hasOverride := Value(nil), false
		if isInstance {
			attribute, hasOverride = instance.class.lookup(name)
			if descriptor, ok := attribute.(*descriptorValue); !ok || descriptor.kind != propertyDescriptor {
				hasOverride = false
			}
		}
		if !hasOverride {
			return runtimeTypeOf(owner), true
		}
	}
	if iterator, ok := owner.(valueIterator); ok {
		switch name {
		case "__iter__":
			return nativeFunctionNamed(iterator.TypeName()+".__iter__", 0, 0,
				func(_ *frame, _ []Value) (Value, *Exception, error) {
					return iterator, nil, nil
				}), true
		case "__next__":
			return nativeFunctionNamed(iterator.TypeName()+".__next__", 0, 0,
				func(caller *frame, _ []Value) (Value, *Exception, error) {
					return builtinNext(caller, []Value{iterator})
				}), true
		}
	}
	switch owner := owner.(type) {
	case attributeValue:
		return owner.attribute(name)
	case *Module:
		if name == "__dict__" {
			return &namespaceValue{namespace: owner.globals}, true
		}
		return owner.globals.get(name)
	case *typeValue:
		if value, found := intrinsicTypeAttribute(owner, name); found {
			return value, true
		}
		value, found, fromMetaclass := lookupTypeAttribute(owner, name)
		if fromMetaclass {
			return bindCallable(value, owner), true
		}
		return bindClassAttribute(value, owner), found
	case *instanceValue:
		if name == "__dict__" {
			return &namespaceValue{namespace: owner.attributes}, true
		}
		if value, found := owner.attributes.get(name); found {
			return value, true
		}
		value, found := owner.class.lookup(name)
		if !found && owner.sequence != nil {
			return listMethod(owner.sequence, name)
		}
		if !found && owner.mapping != nil {
			return owner.mapping.attribute(name)
		}
		if !found {
			return nil, false
		}
		if descriptor, ok := value.(*memberDescriptorValue); ok {
			return owner.attributes.get(descriptor.name)
		}
		if descriptor, ok := value.(*descriptorValue); ok {
			switch descriptor.kind {
			case classMethodDescriptor:
				return bindDescriptorCallable(descriptor.callable, owner.class), true
			case staticMethodDescriptor:
				return descriptor.callable, true
			}
		}
		return bindCallable(value, owner), true
	default:
		return nil, false
	}
}

// builtinSetAttr writes attributes on each mutable runtime object representation.
func builtinSetAttr(caller *frame, arguments []Value) (Value, *Exception, error) {
	name, ok := arguments[1].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "attribute name must be string"), nil
	}
	switch owner := arguments[0].(type) {
	case *Module:
		owner.globals.values[name.value] = arguments[2]
	case *typeValue:
		setTypeAttribute(owner, name.value, arguments[2])
	case *instanceValue:
		return setInstanceAttribute(caller, owner, name.value, arguments[2])
	case *functionValue:
		setFunctionAttribute(owner, name.value, arguments[2])
	case *nativeFunctionValue:
		if owner.attributes == nil {
			owner.attributes = newNamespace()
		}
		owner.attributes.values[name.value] = arguments[2]
	case *descriptorValue:
		owner.attributes.values[name.value] = arguments[2]
	case *weakReferenceValue:
		owner.attributes.values[name.value] = arguments[2]
	case *fileValue:
		if owner.attributes == nil {
			owner.attributes = newNamespace()
		}
		owner.attributes.values[name.value] = arguments[2]
	case *Exception:
		owner.setAttribute(name.value, arguments[2])
	case *tracebackValue:
		if name.value != "tb_next" || (arguments[2] != None && arguments[2].TypeName() != "traceback") {
			return nil, newException("TypeError", "traceback attribute is not writable"), nil
		}
		owner.next = arguments[2]
	default:
		return nil, newException("AttributeError", "object has no writable attributes"), nil
	}
	return None, nil, nil
}

// builtinDelAttr removes an existing attribute from a mutable runtime object.
func builtinDelAttr(caller *frame, arguments []Value) (Value, *Exception, error) {
	name, ok := arguments[1].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "attribute name must be string"), nil
	}
	var attributes *Namespace
	switch owner := arguments[0].(type) {
	case *Module:
		attributes = owner.globals
	case *typeValue:
		attributes = owner.namespace
	case *instanceValue:
		return deleteInstanceAttribute(caller, owner, name.value)
	case *functionValue:
		attributes = owner.attributes
	case *nativeFunctionValue:
		attributes = owner.attributes
	case *descriptorValue:
		attributes = owner.attributes
	}
	if attributes == nil {
		return nil, newException("AttributeError", "object has no writable attributes"), nil
	}
	if _, found := attributes.values[name.value]; !found {
		return nil, newException("AttributeError", "object has no attribute '"+name.value+"'"), nil
	}
	delete(attributes.values, name.value)
	return None, nil, nil
}

func setInstanceAttribute(
	caller *frame,
	instance *instanceValue,
	name string,
	value Value,
) (Value, *Exception, error) {
	setter, found := instance.class.lookup("__setattr__")
	if native, ok := setter.(*nativeFunctionValue); ok && native.name == "object.__setattr__" {
		found = false
	}
	if found {
		return callValueSynchronously(
			caller,
			bindCallable(setter, instance),
			[]Value{&stringValue{value: name}, value},
		)
	}
	return builtinObjectSetAttr(caller, []Value{
		instance, &stringValue{value: name}, value,
	})
}

func deleteInstanceAttribute(
	caller *frame,
	instance *instanceValue,
	name string,
) (Value, *Exception, error) {
	deleter, found := instance.class.lookup("__delattr__")
	if native, ok := deleter.(*nativeFunctionValue); ok && native.name == "object.__delattr__" {
		found = false
	}
	if found {
		return callValueSynchronously(
			caller,
			bindCallable(deleter, instance),
			[]Value{&stringValue{value: name}},
		)
	}
	if _, found := instance.attributes.values[name]; !found {
		return nil, newException(
			"AttributeError", "'"+instance.class.name+"' object has no attribute '"+name+"'",
		), nil
	}
	delete(instance.attributes.values, name)
	return None, nil, nil
}

// setFunctionAttribute keeps structured function metadata synchronized with
// the writable attribute namespace exposed to Python.
func setFunctionAttribute(function *functionValue, name string, value Value) {
	if name == "__defaults__" {
		if defaults, ok := value.(*tupleValue); ok {
			function.defaults = append([]Value(nil), defaults.elements...)
		} else if value == None {
			function.defaults = nil
		}
	}
	if name == "__kwdefaults__" {
		function.keywordDefaults = nil
		if defaults, ok := value.(*dictValue); ok {
			function.keywordDefaults = make(map[string]Value)
			for _, entry := range defaults.entries {
				if key, ok := entry.key.(*stringValue); ok {
					function.keywordDefaults[key.value] = entry.value
				}
			}
		}
	}
	if function.attributes == nil {
		function.attributes = newNamespace()
	}
	function.attributes.values[name] = value
}

// builtinDir returns the known attribute names for the runtime's concrete
// namespace-bearing values. Dynamic __dir__ hooks are not invoked yet.
func builtinDir(caller *frame, arguments []Value) (Value, *Exception, error) {
	if len(arguments) == 1 {
		if instance, ok := arguments[0].(*instanceValue); ok {
			if method, found := instance.class.lookup("__dir__"); found {
				if native, objectDir := method.(*nativeFunctionValue); !objectDir || native.name != "object.__dir__" {
					value, exception, err := callValueSynchronously(
						caller, bindCallable(method, instance), nil,
					)
					if err != nil || exception != nil {
						return nil, exception, err
					}
					elements, exception := iterableElements(value, "dir")
					if exception != nil {
						return nil, exception, nil
					}
					sort.Slice(elements, func(left, right int) bool {
						return elements[left].Repr() < elements[right].Repr()
					})
					return &listValue{elements: elements}, nil, nil
				}
			}
		}
	}
	names := make(map[string]struct{})
	addBuiltinNames := func(typeName string) {
		var methods []string
		switch typeName {
		case "list":
			methods = []string{"append", "clear", "copy", "count", "extend", "index", "insert", "pop", "remove", "reverse", "sort", "__contains__", "__getitem__", "__iter__", "__len__", "__setitem__"}
		case "dict":
			methods = []string{"clear", "copy", "fromkeys", "get", "items", "keys", "pop", "popitem", "setdefault", "update", "values", "__contains__", "__getitem__", "__iter__", "__len__", "__setitem__"}
		case "tuple":
			methods = []string{"count", "index", "__contains__", "__getitem__", "__iter__", "__len__"}
		case "str":
			methods = []string{"capitalize", "casefold", "count", "endswith", "find", "format", "isalnum", "isascii", "isidentifier", "isspace", "isupper", "join", "lower", "lstrip", "partition", "removeprefix", "removesuffix", "replace", "rfind", "rsplit", "rstrip", "split", "splitlines", "startswith", "strip", "upper"}
		case "int":
			methods = []string{"__abs__", "__add__", "__and__", "__bool__", "__ceil__", "__divmod__", "__float__", "__floor__", "__floordiv__", "__index__", "__int__", "__invert__", "__lshift__", "__mod__", "__mul__", "__neg__", "__or__", "__pos__", "__pow__", "__rshift__", "__sub__", "__truediv__", "__trunc__", "__xor__", "bit_count", "bit_length", "to_bytes"}
		case "code":
			methods = []string{"co_argcount", "co_cellvars", "co_code", "co_consts", "co_exceptiontable", "co_filename", "co_firstlineno", "co_flags", "co_freevars", "co_kwonlyargcount", "co_lines", "co_linetable", "co_name", "co_names", "co_nlocals", "co_positions", "co_posonlyargcount", "co_qualname", "co_stacksize", "co_varnames", "replace"}
		case "StringIO", "BytesIO", "TextIOWrapper", "BufferedIOBase", "IOBase":
			methods = []string{"close", "closed", "fileno", "flush", "isatty", "read", "readable", "readline", "readlines", "seek", "seekable", "tell", "truncate", "writable", "write", "writelines", "__enter__", "__exit__", "__iter__", "__next__"}
		}
		for _, name := range methods {
			names[name] = struct{}{}
		}
	}
	if len(arguments) == 0 {
		for name := range caller.locals.values {
			names[name] = struct{}{}
		}
	} else {
		switch value := arguments[0].(type) {
		case *typeValue:
			for _, class := range value.methodResolutionOrder() {
				for name := range class.namespace.values {
					names[name] = struct{}{}
				}
			}
			for _, name := range []string{
				"__bases__", "__dict__", "__doc__", "__flags__", "__module__",
				"__mro__", "__name__", "__qualname__",
			} {
				names[name] = struct{}{}
			}
			if value.builtinBase != nil {
				addBuiltinNames(value.builtinBase.name)
			}
		case *builtinTypeValue:
			addBuiltinNames(value.name)
			for _, name := range []string{"__bases__", "__dict__", "__doc__", "__module__", "__mro__", "__name__", "__qualname__"} {
				names[name] = struct{}{}
			}
		case *instanceValue:
			for name := range value.attributes.values {
				names[name] = struct{}{}
			}
			for _, class := range value.class.methodResolutionOrder() {
				for name := range class.namespace.values {
					names[name] = struct{}{}
				}
			}
			if value.class.builtinBase != nil {
				addBuiltinNames(value.class.builtinBase.name)
			}
		case *listValue:
			addBuiltinNames("list")
		case *tupleValue:
			addBuiltinNames("tuple")
		case *dictValue:
			addBuiltinNames("dict")
		case *stringValue:
			addBuiltinNames("str")
		case *intValue, *boolValue:
			addBuiltinNames("int")
		case *functionValue:
			if value.attributes != nil {
				for name := range value.attributes.values {
					names[name] = struct{}{}
				}
			}
			for _, name := range []string{"__call__", "__code__", "__defaults__", "__dict__", "__globals__", "__kwdefaults__", "__module__", "__name__", "__qualname__"} {
				names[name] = struct{}{}
			}
		case *Module:
			for name := range value.globals.values {
				names[name] = struct{}{}
			}
		case *namespaceValue:
			for name := range value.namespace.values {
				names[name] = struct{}{}
			}
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	values := make([]Value, len(ordered))
	for index, name := range ordered {
		values[index] = &stringValue{value: name}
	}
	return &listValue{elements: values}, nil, nil
}

// builtinLen returns the logical size of each supported built-in collection.
func builtinLen(caller *frame, arguments []Value) (Value, *Exception, error) {
	var length int
	if instance, ok := arguments[0].(*instanceValue); ok {
		method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__len__")
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if found {
			result, exception, err := callValueSynchronously(caller, method, nil)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			integer, ok := integerOperand(result)
			if !ok || !integer.IsInt64() {
				return nil, newException("TypeError", "__len__ returned non-int"), nil
			}
			if integer.Sign() < 0 {
				return nil, newException("ValueError", "__len__() should return >= 0"), nil
			}
			return &intValue{value: integer}, nil, nil
		}
	}
	switch value := arguments[0].(type) {
	case *stringValue:
		length = len([]rune(value.value))
	case *bytesValue:
		length = len(value.value)
	case *bytearrayValue:
		length = len(value.value)
	case *tupleValue:
		length = len(value.elements)
	case *listValue:
		length = len(value.elements)
	case *dictValue:
		length = len(value.entries)
	case *setValue:
		length = len(value.entries)
	case *frozenSetValue:
		length = len(value.entries)
	case *dictViewValue:
		length = len(value.dictionary.entries)
	case *dequeValue:
		length = len(value.elements)
	case *instanceValue:
		switch {
		case value.sequence != nil:
			length = len(value.sequence.elements)
		case value.tuple != nil:
			length = len(value.tuple.elements)
		case value.mapping != nil:
			length = len(value.mapping.entries)
		default:
			return nil, newException("TypeError", "object of type '"+arguments[0].TypeName()+"' has no len()"), nil
		}
	default:
		return nil, newException("TypeError", "object of type '"+arguments[0].TypeName()+"' has no len()"), nil
	}
	return newInt64(int64(length)), nil, nil
}

func builtinIsInstance(caller *frame, arguments []Value) (Value, *Exception, error) {
	matches, exception := runtimeIsInstance(caller.runtime, arguments[0], arguments[1])
	return pythonBool(matches), exception, nil
}

// runtimeIsInstance evaluates tuple, concrete, and ABC-backed class information.
func runtimeIsInstance(runtime *Runtime, value, classInfo Value) (bool, *Exception) {
	if tuple, ok := classInfo.(*tupleValue); ok {
		for _, candidate := range tuple.elements {
			matches, exception := runtimeIsInstance(runtime, value, candidate)
			if exception != nil || matches {
				return matches, exception
			}
		}
		return false, nil
	}
	if instance, ok := value.(*instanceValue); ok {
		if specified, found := instance.attributes.get("_spec_class"); found && specified != None {
			switch requested := classInfo.(type) {
			case *typeValue:
				if specifiedClass, ok := specified.(*typeValue); ok && specifiedClass.isSubclassOf(requested) {
					return true, nil
				}
			case *builtinTypeValue:
				if specified == requested {
					return true, nil
				}
				if specifiedClass, ok := specified.(*typeValue); ok && specifiedClass.inheritedBuiltinBase() == requested {
					return true, nil
				}
			}
		}
	}
	if class, ok := classInfo.(*typeValue); ok {
		if descriptor, descriptorOK := value.(*descriptorValue); descriptorOK {
			return descriptor.class.isSubclassOf(class), nil
		}
		return runtime.abcInstanceMatches(class, value), nil
	}
	return valueIsInstance(value, classInfo)
}

func builtinIsSubclass(caller *frame, arguments []Value) (Value, *Exception, error) {
	matches, exception := runtimeIsSubclass(caller.runtime, arguments[0], arguments[1])
	return pythonBool(matches), exception, nil
}

// runtimeIsSubclass evaluates concrete inheritance and runtime-local ABC registration.
func runtimeIsSubclass(runtime *Runtime, candidate, classInfo Value) (bool, *Exception) {
	if tuple, ok := classInfo.(*tupleValue); ok {
		for _, parent := range tuple.elements {
			matches, exception := runtimeIsSubclass(runtime, candidate, parent)
			if exception != nil || matches {
				return matches, exception
			}
		}
		return false, nil
	}
	if parent, ok := classInfo.(*exceptionTypeValue); ok {
		switch candidate := candidate.(type) {
		case *exceptionTypeValue:
			return candidate.isSubclassOf(parent), nil
		case *typeValue:
			base := candidate.builtinExceptionBase()
			return base != nil && base.isSubclassOf(parent), nil
		default:
			return false, newException("TypeError", "issubclass() arg 1 must be a class")
		}
	}
	if parent, ok := classInfo.(*typeValue); ok {
		if !isClassValue(candidate) {
			return false, newException("TypeError", "issubclass() arg 1 must be a class")
		}
		return runtime.abcSubclassMatches(parent, candidate), nil
	}
	parent, parentOK := classInfo.(*builtinTypeValue)
	if !parentOK || !isClassValue(candidate) {
		return false, newException("TypeError", "issubclass() arguments must be classes")
	}
	if candidate == parent {
		return true, nil
	}
	if class, ok := candidate.(*typeValue); ok {
		return class.inheritedBuiltinBase() == parent, nil
	}
	return false, nil
}

func builtinCallable(_ *frame, arguments []Value) (Value, *Exception, error) {
	callable := false
	switch value := arguments[0].(type) {
	case *nativeFunctionValue, *functionValue, *boundMethodValue, *genericBoundMethodValue,
		*boundNativeMethodValue, *buildClassValue, *typeValue,
		*builtinTypeValue, *exceptionTypeValue, *partialValue:
		callable = true
	case *weakReferenceValue:
		callable = true
	case *instanceValue:
		_, callable = value.class.lookup("__call__")
	}
	return pythonBool(callable), nil, nil
}

// builtinAbs preserves arbitrary-precision integers and computes magnitudes
// for the runtime's boolean, float, and complex numeric representations.
func builtinAbs(caller *frame, arguments []Value) (Value, *Exception, error) {
	switch value := arguments[0].(type) {
	case *intValue:
		var absolute big.Int
		absolute.Abs(&value.value)
		return &intValue{value: absolute}, nil, nil
	case *boolValue:
		if value.value {
			return newInt64(1), nil, nil
		}
		return newInt64(0), nil, nil
	case *floatValue:
		return &floatValue{value: math.Abs(value.value)}, nil, nil
	case *complexValue:
		return &floatValue{value: math.Hypot(value.real, value.imaginary)}, nil, nil
	default:
		if instance, ok := arguments[0].(*instanceValue); ok {
			if method, found := instance.class.lookup("__abs__"); found {
				return callValueSynchronously(caller, bindCallable(method, instance), nil)
			}
		}
		return nil, newException("TypeError", "bad operand type for abs(): '"+value.TypeName()+"'"), nil
	}
}

// builtinRound dispatches user-defined rounding and implements integer and
// floating-point rounding with an optional decimal precision.
func builtinRound(caller *frame, arguments []Value) (Value, *Exception, error) {
	number, ok := numericFloat(arguments[0])
	if !ok {
		if instance, isInstance := arguments[0].(*instanceValue); isInstance {
			method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__round__")
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if found {
				return callValueSynchronously(caller, method, arguments[1:])
			}
		}
		return nil, newException("TypeError", "type does not define __round__ method"), nil
	}
	if len(arguments) == 1 {
		return newInt64(int64(math.RoundToEven(number))), nil, nil
	}
	digits, integerDigits := integerOperand(arguments[1])
	if !integerDigits || !digits.IsInt64() {
		return nil, newException("TypeError", "ndigits must be an integer"), nil
	}
	scale := math.Pow10(int(digits.Int64()))
	return &floatValue{value: math.RoundToEven(number*scale) / scale}, nil, nil
}

func builtinBaseRepresentation(base int, prefix string) nativeFunction {
	return func(caller *frame, arguments []Value) (Value, *Exception, error) {
		value, exception, err := operatorIndex(caller, arguments)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		var integer big.Int
		integer.Set(&value.(*intValue).value)
		negative := integer.Sign() < 0
		if negative {
			integer.Neg(&integer)
		}
		text := prefix + integer.Text(base)
		if negative {
			text = "-" + text
		}
		return &stringValue{value: text}, nil, nil
	}
}

func builtinAll(caller *frame, arguments []Value) (Value, *Exception, error) {
	return reduceTruth(caller, arguments[0], true)
}

func builtinAny(caller *frame, arguments []Value) (Value, *Exception, error) {
	return reduceTruth(caller, arguments[0], false)
}

// reduceTruth implements the shared short-circuit core for all() and any().
func reduceTruth(caller *frame, iterable Value, all bool) (Value, *Exception, error) {
	iterator, ok := newIterator(iterable)
	if !ok {
		return nil, newException("TypeError", "argument is not iterable"), nil
	}
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			return pythonBool(all), nil, nil
		}
		truth, exception, err := truthValueForFrame(caller, value)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if truth != all {
			return pythonBool(!all), nil, nil
		}
	}
}

// builtinNext resumes VM-backed generators or advances a concrete iterator,
// returning an explicit default or raising StopIteration on exhaustion.
func builtinNext(caller *frame, arguments []Value) (Value, *Exception, error) {
	iterator, ok := arguments[0].(valueIterator)
	if !ok {
		if instance, instanceOK := arguments[0].(*instanceValue); instanceOK {
			method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__next__")
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if found {
				value, exception, err := callValueSynchronously(caller, method, nil)
				if exception != nil && exception.class == stopIterationType && len(arguments) == 2 {
					return arguments[1], nil, nil
				}
				return value, exception, err
			}
		}
		return nil, newException("TypeError", "object is not an iterator"), nil
	}
	var value Value
	var available bool
	var exception *Exception
	var err error
	if generator, ok := iterator.(*generatorValue); ok {
		value, available, exception, err = resumeGeneratorFrom(caller.runtime, caller, generator)
	} else {
		value, available, exception, err = nextNativeIterator(caller.runtime, iterator)
	}
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if available {
		return value, nil, nil
	}
	if len(arguments) == 2 {
		return arguments[1], nil, nil
	}
	return nil, newException("StopIteration", ""), nil
}

// builtinEnumerate eagerly pairs iterable values with consecutive integer indexes.
func builtinEnumerate(caller *frame, arguments []Value) (Value, *Exception, error) {
	iterator, exception, err := newIteratorForFrame(caller, arguments[0])
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if iterator == nil {
		return nil, newException("TypeError", "argument is not iterable"), nil
	}
	start := int64(0)
	if len(arguments) == 2 {
		integer, integerOK := integerOperand(arguments[1])
		if !integerOK || !integer.IsInt64() {
			return nil, newException("TypeError", "start must be an integer"), nil
		}
		start = integer.Int64()
	}
	values := make([]Value, 0)
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			break
		}
		values = append(values, &tupleValue{elements: []Value{newInt64(start), value}})
		start++
	}
	return &sequenceIterator{sequence: &listValue{elements: values}}, nil, nil
}

// builtinMap eagerly applies a supported callable across parallel iterables.
func builtinMap(caller *frame, arguments []Value) (Value, *Exception, error) {
	callable := arguments[0]
	iterators := make([]valueIterator, len(arguments)-1)
	for index, argument := range arguments[1:] {
		iterator, exception, err := newIteratorForFrame(caller, argument)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if iterator == nil {
			return nil, newException("TypeError", "map argument is not iterable"), nil
		}
		iterators[index] = iterator
	}
	results := make([]Value, 0)
	for {
		values := make([]Value, len(iterators))
		for index, iterator := range iterators {
			value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if !available {
				return &sequenceIterator{sequence: &listValue{elements: results}}, nil, nil
			}
			values[index] = value
		}
		value, exception, err := directBuiltinCall(caller, callable, values)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		results = append(results, value)
	}
}

// builtinFilter eagerly evaluates its predicate and returns an iterator over
// the retained values. Eagerness is not observable to the current VM subset.
func builtinFilter(caller *frame, arguments []Value) (Value, *Exception, error) {
	predicate := arguments[0]
	iterator, ok := newIterator(arguments[1])
	if !ok {
		return nil, newException("TypeError", "filter argument is not iterable"), nil
	}
	values := make([]Value, 0)
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			return &sequenceIterator{sequence: &listValue{elements: values}}, nil, nil
		}
		keep := value
		if predicate != None {
			keep, exception, err = directBuiltinCall(caller, predicate, []Value{value})
			if err != nil || exception != nil {
				return nil, exception, err
			}
		}
		truth, exception, err := truthValueForFrame(caller, keep)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if truth {
			values = append(values, value)
		}
	}
}

// directBuiltinCall invokes a Go-backed callable without creating a VM call frame.
func directBuiltinCall(caller *frame, callable Value, arguments []Value) (Value, *Exception, error) {
	return callValueSynchronously(caller, callable, arguments)
}

func builtinOrd(_ *frame, arguments []Value) (Value, *Exception, error) {
	switch value := arguments[0].(type) {
	case *stringValue:
		runes := []rune(value.value)
		if len(runes) == 1 {
			return newInt64(int64(runes[0])), nil, nil
		}
	case *bytesValue:
		if len(value.value) == 1 {
			return newInt64(int64(value.value[0])), nil, nil
		}
	}
	return nil, newException("TypeError", "ord() expected a character"), nil
}

func builtinChr(_ *frame, arguments []Value) (Value, *Exception, error) {
	integer, ok := integerOperand(arguments[0])
	if !ok || !integer.IsInt64() {
		return nil, newException("TypeError", "an integer is required"), nil
	}
	value := integer.Int64()
	if value < 0 || value > 0x10ffff {
		return nil, newException("ValueError", "chr() arg not in range(0x110000)"), nil
	}
	return &stringValue{value: string(rune(value))}, nil, nil
}

func builtinID(_ *frame, arguments []Value) (Value, *Exception, error) {
	pointer := reflect.ValueOf(arguments[0])
	if pointer.Kind() == reflect.Pointer {
		return newInt64(int64(pointer.Pointer())), nil, nil
	}
	return newInt64(0), nil, nil
}

// builtinRepr dispatches a user-defined __repr__ and validates its string result.
func builtinRepr(caller *frame, arguments []Value) (Value, *Exception, error) {
	if instance, ok := arguments[0].(*instanceValue); ok {
		if method, found := instance.class.lookup("__repr__"); found {
			if resolved, descriptor, exception, err := resolvePythonDescriptor(
				caller, method, instance, instance.class,
			); descriptor {
				if err != nil || exception != nil {
					return nil, exception, err
				}
				method = resolved
			}
			value, exception, err := callValueSynchronously(
				caller, bindCallable(method, instance), nil,
			)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if _, ok := value.(*stringValue); !ok {
				return nil, newException("TypeError", "__repr__ returned non-string"), nil
			}
			return value, nil, nil
		}
	}
	if text, handled, exception, err := containerRepr(caller, arguments[0]); handled {
		return text, exception, err
	}
	return &stringValue{value: arguments[0].Repr()}, nil, nil
}

// containerRepr recursively renders the supported sequence containers through
// Python-level repr calls while retaining their delimiters.
func containerRepr(
	caller *frame,
	value Value,
) (Value, bool, *Exception, error) {
	var elements []Value
	prefix, suffix := "", ""
	singletonSuffix := ""
	switch sequence := value.(type) {
	case *listValue:
		elements, prefix, suffix = sequence.elements, "[", "]"
	case *tupleValue:
		elements, prefix, suffix = sequence.elements, "(", ")"
		singletonSuffix = ","
	default:
		return nil, false, nil, nil
	}
	parts := make([]string, len(elements))
	for index, element := range elements {
		representation, exception, err := builtinRepr(caller, []Value{element})
		if err != nil || exception != nil {
			return nil, true, exception, err
		}
		parts[index] = representation.(*stringValue).value
	}
	content := strings.Join(parts, ", ")
	if len(elements) == 1 {
		content += singletonSuffix
	}
	return &stringValue{value: prefix + content + suffix}, true, nil, nil
}

// builtinHash dispatches user hashes and supplies stable hashes for current native values.
func builtinHash(caller *frame, arguments []Value) (Value, *Exception, error) {
	if reference, ok := arguments[0].(*weakReferenceValue); ok {
		return builtinHash(caller, []Value{reference.referent})
	}
	if instance, ok := arguments[0].(*instanceValue); ok {
		if method, found, lookupException, lookupErr := lookupBoundSpecialMethod(caller, instance, "__hash__"); lookupErr != nil || lookupException != nil {
			return nil, lookupException, lookupErr
		} else if found {
			if native, ok := method.(*nativeFunctionValue); ok && native.name == "object.__hash__" {
				return newInt64(int64(reflect.ValueOf(instance).Pointer())), nil, nil
			}
			if native, ok := method.(*boundNativeMethodValue); ok && native.function.name == "object.__hash__" {
				return newInt64(int64(reflect.ValueOf(instance).Pointer())), nil, nil
			}
			if method == None {
				return nil, newException("TypeError", "unhashable type: '"+instance.TypeName()+"'"), nil
			}
			value, exception, err := callValueSynchronously(
				caller, method, nil,
			)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if _, ok := integerOperand(value); !ok {
				return nil, newException("TypeError", "__hash__ method should return an integer"), nil
			}
			return value, nil, nil
		}
	}
	if unhashable, found := unhashableComponent(arguments[0]); found {
		return nil, newException("TypeError", "unhashable type: '"+unhashable+"'"), nil
	}
	pointer := reflect.ValueOf(arguments[0])
	if pointer.Kind() == reflect.Pointer {
		switch arguments[0].(type) {
		case *instanceValue, *objectValue:
			return newInt64(int64(pointer.Pointer())), nil, nil
		}
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(arguments[0].Repr()))
	return newInt64(int64(hasher.Sum64())), nil, nil
}

func builtinMax(caller *frame, arguments []Value, keywords *dictValue) (Value, *Exception, error) {
	return builtinExtremum(caller, arguments, keywords, true)
}

func builtinMin(caller *frame, arguments []Value, keywords *dictValue) (Value, *Exception, error) {
	return builtinExtremum(caller, arguments, keywords, false)
}

// builtinExtremum selects the minimum or maximum ordered value from its inputs.
func builtinExtremum(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
	maximum bool,
) (Value, *Exception, error) {
	key := Value(None)
	var defaultValue Value
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "keywords must be strings"), nil
			}
			switch name.value {
			case "key":
				key = entry.value
			case "default":
				if len(arguments) != 1 {
					return nil, newException("TypeError", "default is only allowed with an iterable"), nil
				}
				defaultValue = entry.value
			default:
				return nil, newException("TypeError", "unexpected keyword argument '"+name.value+"'"), nil
			}
		}
	}
	var iterator valueIterator
	if len(arguments) == 1 {
		var ok bool
		iterator, ok = newIterator(arguments[0])
		if !ok {
			return nil, newException("TypeError", "argument is not iterable"), nil
		}
	} else {
		iterator = &sequenceIterator{sequence: &tupleValue{elements: arguments}}
	}
	var selected, selectedKey Value
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			break
		}
		if selected == nil {
			selected = value
			selectedKey = value
			if key != None {
				selectedKey, exception, err = callValueSynchronously(caller, key, []Value{value})
				if err != nil || exception != nil {
					return nil, exception, err
				}
			}
			continue
		}
		valueKey := value
		if key != None {
			valueKey, exception, err = callValueSynchronously(caller, key, []Value{value})
			if err != nil || exception != nil {
				return nil, exception, err
			}
		}
		comparison, ordered, supported := orderedValues(valueKey, selectedKey)
		if !supported || !ordered {
			return nil, newException("TypeError", "values are not orderable"), nil
		}
		if (maximum && comparison > 0) || (!maximum && comparison < 0) {
			selected = value
			selectedKey = valueKey
		}
	}
	if selected == nil {
		if defaultValue != nil {
			return defaultValue, nil, nil
		}
		return nil, newException("ValueError", "arg is an empty sequence"), nil
	}
	return selected, nil, nil
}

// builtinSum accumulates integer values from an iterable with an optional start.
func builtinSum(caller *frame, arguments []Value) (Value, *Exception, error) {
	var total big.Int
	if len(arguments) == 2 {
		start, ok := integerOperand(arguments[1])
		if !ok {
			return nil, newException("TypeError", "sum currently requires integer values"), nil
		}
		total.Set(&start)
	}
	iterator, ok := newIterator(arguments[0])
	if !ok {
		return nil, newException("TypeError", "sum() argument is not iterable"), nil
	}
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			return &intValue{value: total}, nil, nil
		}
		integer, integerOK := integerOperand(value)
		if !integerOK {
			return nil, newException("TypeError", "sum currently requires integer values"), nil
		}
		total.Add(&total, &integer)
	}
}

// builtinDivmod returns paired floor-division and modulo results for real numbers.
func builtinDivmod(caller *frame, arguments []Value) (Value, *Exception, error) {
	leftInteger, leftIsInteger := integerOperand(arguments[0])
	rightInteger, rightIsInteger := integerOperand(arguments[1])
	if leftIsInteger && rightIsInteger {
		if rightInteger.Sign() == 0 {
			return nil, newException("ZeroDivisionError", "integer division or modulo by zero"), nil
		}
		var quotient, remainder big.Int
		quotient.QuoRem(&leftInteger, &rightInteger, &remainder)
		if remainder.Sign() != 0 && remainder.Sign() != rightInteger.Sign() {
			quotient.Sub(&quotient, big.NewInt(1))
			remainder.Add(&remainder, &rightInteger)
		}
		return &tupleValue{elements: []Value{
			&intValue{value: quotient}, &intValue{value: remainder},
		}}, nil, nil
	}
	left, leftOK := numericFloat(arguments[0])
	right, rightOK := numericFloat(arguments[1])
	if !leftOK || !rightOK {
		for _, candidate := range []struct {
			owner    Value
			argument Value
			name     string
		}{{arguments[0], arguments[1], "__divmod__"}, {arguments[1], arguments[0], "__rdivmod__"}} {
			instance, ok := candidate.owner.(*instanceValue)
			if !ok {
				continue
			}
			method, found, exception, err := lookupBoundSpecialMethod(caller, instance, candidate.name)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if found {
				return callValueSynchronously(caller, method, []Value{candidate.argument})
			}
		}
		return nil, newException("TypeError", "unsupported operand type(s) for divmod()"), nil
	}
	if right == 0 {
		return nil, newException("ZeroDivisionError", "float divmod()"), nil
	}
	quotient := math.Floor(left / right)
	remainder := math.Mod(left, right)
	if remainder != 0 && math.Signbit(remainder) != math.Signbit(right) {
		remainder += right
	}
	return &tupleValue{elements: []Value{
		&floatValue{value: quotient}, &floatValue{value: remainder},
	}}, nil, nil
}

// builtinSorted drains and stably orders an iterable with an optional key and reversal.
func builtinSorted(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	key := Value(None)
	reverse := false
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "sorted() keywords must be strings"), nil
			}
			switch name.value {
			case "key":
				key = entry.value
			case "reverse":
				truth, exception, err := truthValueForFrame(caller, entry.value)
				if err != nil || exception != nil {
					return nil, exception, err
				}
				reverse = truth
			default:
				return nil, newException("TypeError", "sorted() got an unexpected keyword argument '"+name.value+"'"), nil
			}
		}
	}
	iterator, ok := newIterator(arguments[0])
	if !ok {
		return nil, newException("TypeError", "sorted() argument is not iterable"), nil
	}
	elements := make([]Value, 0)
	for {
		value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			break
		}
		elements = append(elements, value)
	}
	exception, err := sortValues(caller, elements, key, reverse)
	if err != nil || exception != nil {
		return nil, exception, err
	}
	return &listValue{elements: elements}, nil, nil
}

// builtinEval compiles one expression and executes it in a selected namespace.
func (runtimeState *Runtime) builtinEval(
	caller *frame,
	arguments []Value,
) (Value, *Exception, error) {
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "eval() arg 1 must be a string"), nil
	}
	if runtimeState.compiler == nil {
		return nil, newException("RuntimeError", "dynamic compilation is not configured"), nil
	}
	code, err := runtimeState.compiler(
		"<string>",
		"__bullsnake_eval_result__ = ("+text.value+")\n",
	)
	if err != nil {
		return nil, nil, err
	}
	prepared, err := runtimeState.prepare(code)
	if err != nil {
		return nil, nil, err
	}
	globals, exception := evalNamespace(caller.globals, arguments[1:])
	if exception != nil {
		return nil, exception, nil
	}
	child := &frame{
		runtime:    runtimeState,
		code:       prepared,
		stack:      make([]Value, 0, prepared.stackSize),
		fastLocals: make([]Value, len(prepared.locals)),
		locals:     globals,
		globals:    globals,
		builtins:   runtimeState.builtins,
	}
	_, raised, err := execute(&threadState{current: child})
	if err != nil {
		return nil, nil, err
	}
	if raised != nil {
		return nil, raised.exception, nil
	}
	result, found := globals.get("__bullsnake_eval_result__")
	if !found {
		return nil, nil, caller.failure(caller.instruction-1, "eval result is missing")
	}
	delete(globals.values, "__bullsnake_eval_result__")
	return result, nil, nil
}

// builtinExec compiles statements and synchronizes writes to an explicit locals dict.
func (runtimeState *Runtime) builtinExec(
	caller *frame,
	arguments []Value,
) (Value, *Exception, error) {
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "exec() arg 1 must be a string"), nil
	}
	if runtimeState.compiler == nil {
		return nil, newException("RuntimeError", "dynamic compilation is not configured"), nil
	}
	code, err := runtimeState.compiler("<string>", text.value)
	if err != nil {
		return nil, nil, err
	}
	prepared, err := runtimeState.prepare(code)
	if err != nil {
		return nil, nil, err
	}
	globals := caller.globals
	var globalsDictionary *dictValue
	if len(arguments) >= 2 && arguments[1] != None {
		var exception *Exception
		globals, exception = mappingNamespace(arguments[1])
		if exception != nil {
			return nil, exception, nil
		}
		globalsDictionary, _ = arguments[1].(*dictValue)
	}
	locals := globals
	var localsDictionary *dictValue
	if len(arguments) == 3 && arguments[2] != None {
		var exception *Exception
		locals, exception = mappingNamespace(arguments[2])
		if exception != nil {
			return nil, exception, nil
		}
		localsDictionary, _ = arguments[2].(*dictValue)
	} else {
		localsDictionary = globalsDictionary
	}
	child := &frame{
		runtime:    runtimeState,
		code:       prepared,
		stack:      make([]Value, 0, prepared.stackSize),
		fastLocals: make([]Value, len(prepared.locals)),
		locals:     locals,
		globals:    globals,
		builtins:   runtimeState.builtins,
	}
	_, raised, err := execute(&threadState{current: child})
	if err != nil {
		return nil, nil, err
	}
	if raised != nil {
		return nil, raised.exception, nil
	}
	if localsDictionary != nil {
		localsDictionary.entries = nil
		for name, value := range locals.values {
			localsDictionary.entries = append(localsDictionary.entries, dictEntry{
				key: &stringValue{value: name}, value: value,
			})
		}
		localsDictionary.version++
	}
	return None, nil, nil
}

// mappingNamespace converts supported globals/locals mappings into the live
// namespace representation used by frame execution.
func mappingNamespace(value Value) (*Namespace, *Exception) {
	switch mapping := value.(type) {
	case *namespaceValue:
		return mapping.namespace, nil
	case *dictValue:
		namespace := newNamespace()
		for _, entry := range mapping.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "namespace keys must be strings")
			}
			namespace.values[name.value] = entry.value
		}
		return namespace, nil
	case *instanceValue:
		if mapping.mapping == nil {
			return nil, newException("TypeError", "namespace must be a dict")
		}
		namespace := newNamespace()
		for _, entry := range mapping.mapping.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "namespace keys must be strings")
			}
			namespace.values[name.value] = entry.value
		}
		return namespace, nil
	default:
		return nil, newException("TypeError", "namespace must be a dict")
	}
}

// evalNamespace converts the optional eval globals mapping to runtime storage.
func evalNamespace(fallback *Namespace, arguments []Value) (*Namespace, *Exception) {
	if len(arguments) == 0 || arguments[0] == None {
		return fallback, nil
	}
	switch mapping := arguments[0].(type) {
	case *namespaceValue:
		return mapping.namespace, nil
	case *dictValue:
		namespace := newNamespace()
		for _, entry := range mapping.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "globals must have string keys")
			}
			namespace.values[name.value] = entry.value
		}
		return namespace, nil
	default:
		return nil, newException("TypeError", "globals must be a dict")
	}
}

func builtinIter(caller *frame, arguments []Value) (Value, *Exception, error) {
	iterator, exception, err := newIteratorForFrame(caller, arguments[0])
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if iterator == nil {
		return nil, newException(
			"TypeError", "'"+arguments[0].TypeName()+"' object is not iterable",
		), nil
	}
	return iterator, nil, nil
}

func builtinReversed(_ *frame, arguments []Value) (Value, *Exception, error) {
	var elements []Value
	switch sequence := arguments[0].(type) {
	case *listValue:
		elements = sequence.elements
	case *tupleValue:
		elements = sequence.elements
	default:
		return nil, newException("TypeError", "argument to reversed() must be a sequence"), nil
	}
	reversed := make([]Value, len(elements))
	for index := range elements {
		reversed[len(elements)-1-index] = elements[index]
	}
	return &sequenceIterator{sequence: &listValue{elements: reversed}}, nil, nil
}

// constructRange validates arbitrary-size integer bounds and builds a lazy
// range value so even enormous endpoints do not allocate their contents.
func constructRange(arguments []Value) (Value, *Exception) {
	if len(arguments) < 1 || len(arguments) > 3 {
		return nil, newException("TypeError", "range expected 1 to 3 arguments")
	}
	values := make([]big.Int, len(arguments))
	for index, argument := range arguments {
		integer, ok := integerOperand(argument)
		if !ok {
			return nil, newException("TypeError", "'"+argument.TypeName()+"' object cannot be interpreted as an integer")
		}
		values[index].Set(&integer)
	}
	var start, stop, step big.Int
	stop.Set(&values[0])
	step.SetInt64(1)
	if len(values) >= 2 {
		start.Set(&values[0])
		stop.Set(&values[1])
	}
	if len(values) == 3 {
		step.Set(&values[2])
	}
	if step.Sign() == 0 {
		return nil, newException("ValueError", "range() arg 3 must not be zero")
	}
	return &rangeValue{start: start, stop: stop, step: step}, nil
}

// builtinZip consumes the currently synchronous iterables into tuple rows and
// returns an iterator that stops with the shortest input.
func builtinZip(caller *frame, arguments []Value) (Value, *Exception, error) {
	if len(arguments) == 0 {
		return &sequenceIterator{sequence: &listValue{}}, nil, nil
	}
	iterators := make([]valueIterator, len(arguments))
	for index, argument := range arguments {
		iterator, exception, err := newIteratorForFrame(caller, argument)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if iterator == nil {
			return nil, newException("TypeError", "zip argument is not iterable"), nil
		}
		iterators[index] = iterator
	}
	rows := make([]Value, 0)
	for {
		row := make([]Value, len(iterators))
		for index, iterator := range iterators {
			value, available, exception, err := nextNativeIterator(caller.runtime, iterator)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if !available {
				return &sequenceIterator{sequence: &listValue{elements: rows}}, nil, nil
			}
			row[index] = value
		}
		rows = append(rows, &tupleValue{elements: row})
	}
}
