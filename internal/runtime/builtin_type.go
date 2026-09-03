package runtime

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

type builtinTypeValue struct {
	name    string
	matches func(Value) bool
}

func (*builtinTypeValue) TypeName() string { return "type" }
func (class *builtinTypeValue) Repr() string {
	return "<class '" + class.name + "'>"
}
func (*builtinTypeValue) isValue() {}

// attribute resolves metadata and descriptors exposed by a built-in class.
func (class *builtinTypeValue) attribute(name string) (Value, bool) {
	switch name {
	case "__name__", "__qualname__":
		return &stringValue{value: class.name}, true
	case "__module__":
		return &stringValue{value: "builtins"}, true
	case "__doc__":
		return None, true
	case "__mro__":
		values := []Value{class}
		if class.name != "object" {
			values = append(values, builtinTypeNamed("object"))
		}
		return &tupleValue{elements: values}, true
	case "__bases__":
		if class.name == "object" {
			return &tupleValue{}, true
		}
		return &tupleValue{elements: []Value{builtinTypeNamed("object")}}, true
	case "__dict__":
		namespace := newNamespace()
		if class.name == "type" {
			for _, descriptorName := range []string{"__annotations__", "__mro__", "__dict__", "__bases__"} {
				namespace.values[descriptorName] = &typeMetadataDescriptorValue{name: descriptorName}
			}
		}
		return &namespaceValue{namespace: namespace}, true
	default:
		if value, found := builtinTypeMethod(class.name, name); found {
			return value, true
		}
		if class.name != "object" {
			return builtinTypeMethod("object", name)
		}
		return nil, false
	}
}

type typeMetadataDescriptorValue struct {
	name string
}

func (*typeMetadataDescriptorValue) TypeName() string { return "getset_descriptor" }
func (descriptor *typeMetadataDescriptorValue) Repr() string {
	return "<attribute '" + descriptor.name + "' of 'type' objects>"
}
func (*typeMetadataDescriptorValue) isValue() {}

// materializeClassAnnotations evaluates and caches one class's lazy annotation
// function using annotationlib's VALUE format.
func materializeClassAnnotations(
	caller *frame,
	class *typeValue,
) (Value, *Exception, error) {
	if annotations, found := class.namespace.get("__annotations__"); found {
		return annotations, nil, nil
	}
	annotations := Value(&dictValue{})
	if annotate, found := class.namespace.get("__annotate__"); found && annotate != None {
		value, exception, err := callValueSynchronously(caller, annotate, []Value{newInt64(1)})
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if _, ok := value.(*dictValue); !ok {
			return nil, newException("ValueError", "__annotate__ returned a non-dict"), nil
		}
		annotations = value
	}
	class.namespace.values["__annotations__"] = annotations
	return annotations, nil, nil
}

// attribute exposes the bound getter for one type metadata descriptor.
func (descriptor *typeMetadataDescriptorValue) attribute(name string) (Value, bool) {
	if name == "__name__" {
		return &stringValue{value: descriptor.name}, true
	}
	if name == "__objclass__" {
		return builtinTypeNamed("type"), true
	}
	if name != "__get__" {
		return nil, false
	}
	return nativeFunctionNamed("type."+descriptor.name+".__get__", 1, 2,
		func(caller *frame, arguments []Value) (Value, *Exception, error) {
			if class, ok := arguments[0].(*typeValue); ok {
				if descriptor.name == "__annotations__" {
					return materializeClassAnnotations(caller, class)
				}
				if value, found := intrinsicTypeAttribute(class, descriptor.name); found {
					return value, nil, nil
				}
			}
			if class, ok := arguments[0].(*builtinTypeValue); ok {
				switch descriptor.name {
				case "__annotations__":
					return &dictValue{}, nil, nil
				case "__mro__":
					values := []Value{class}
					if class.name != "object" {
						values = append(values, builtinTypeNamed("object"))
					}
					return &tupleValue{elements: values}, nil, nil
				case "__bases__":
					if class.name == "object" {
						return &tupleValue{}, nil, nil
					}
					return &tupleValue{elements: []Value{builtinTypeNamed("object")}}, nil, nil
				case "__dict__":
					value, _ := class.attribute("__dict__")
					return value, nil, nil
				}
			}
			return nil, newException("TypeError", "descriptor does not apply to object"), nil
		}), true
}

var _ Value = (*typeMetadataDescriptorValue)(nil)

var builtinTypes = []*builtinTypeValue{
	{name: "object", matches: func(Value) bool { return true }},
	{name: "NoneType", matches: func(value Value) bool { return value == None }},
	{name: "str", matches: func(value Value) bool { _, ok := value.(*stringValue); return ok }},
	{name: "bytes", matches: func(value Value) bool { _, ok := value.(*bytesValue); return ok }},
	{name: "bytearray", matches: func(value Value) bool { _, ok := value.(*bytearrayValue); return ok }},
	{name: "bool", matches: func(value Value) bool { _, ok := value.(*boolValue); return ok }},
	{name: "int", matches: func(value Value) bool {
		switch value.(type) {
		case *intValue, *boolValue:
			return true
		default:
			return false
		}
	}},
	{name: "float", matches: func(value Value) bool { _, ok := value.(*floatValue); return ok }},
	{name: "complex", matches: func(value Value) bool { _, ok := value.(*complexValue); return ok }},
	{name: "tuple", matches: func(value Value) bool { _, ok := value.(*tupleValue); return ok }},
	{name: "list", matches: func(value Value) bool { _, ok := value.(*listValue); return ok }},
	{name: "dict", matches: func(value Value) bool { _, ok := value.(*dictValue); return ok }},
	{name: "set", matches: func(value Value) bool { _, ok := value.(*setValue); return ok }},
	{name: "frozenset", matches: func(value Value) bool { _, ok := value.(*frozenSetValue); return ok }},
	{name: "range", matches: func(value Value) bool { _, ok := value.(*rangeValue); return ok }},
	{name: "slice", matches: func(value Value) bool { _, ok := value.(*sliceValue); return ok }},
	{name: "memoryview", matches: func(Value) bool { return false }},
	{name: "type", matches: func(value Value) bool {
		switch value.(type) {
		case *builtinTypeValue, *exceptionTypeValue, *typeValue:
			return true
		default:
			return false
		}
	}},
}

var opaqueBuiltinTypes = struct {
	function  *builtinTypeValue
	method    *builtinTypeValue
	ellipsis  *builtinTypeValue
	iterator  *builtinTypeValue
	coroutine *builtinTypeValue
	generator *builtinTypeValue
	asyncGen  *builtinTypeValue
	getSet    *builtinTypeValue
	member    *builtinTypeValue
}{
	function: &builtinTypeValue{name: "function", matches: func(value Value) bool { _, ok := value.(*functionValue); return ok }},
	method: &builtinTypeValue{name: "method", matches: func(value Value) bool {
		switch value.(type) {
		case *boundMethodValue, *genericBoundMethodValue:
			return true
		default:
			return false
		}
	}},
	ellipsis:  &builtinTypeValue{name: "ellipsis", matches: func(value Value) bool { _, ok := value.(*ellipsisValue); return ok }},
	iterator:  &builtinTypeValue{name: "iterator", matches: func(value Value) bool { _, ok := value.(valueIterator); return ok }},
	coroutine: &builtinTypeValue{name: "coroutine", matches: func(value Value) bool { _, ok := value.(*coroutineValue); return ok }},
	generator: &builtinTypeValue{name: "generator", matches: func(value Value) bool { _, ok := value.(*generatorValue); return ok }},
	asyncGen:  &builtinTypeValue{name: "async_generator", matches: func(value Value) bool { _, ok := value.(*asyncGeneratorValue); return ok }},
	getSet:    &builtinTypeValue{name: "getset_descriptor", matches: func(value Value) bool { _, ok := value.(*typeMetadataDescriptorValue); return ok }},
	member:    &builtinTypeValue{name: "member_descriptor", matches: func(Value) bool { return false }},
}

var mappingProxyType = &builtinTypeValue{
	name:    "mappingproxy",
	matches: func(value Value) bool { _, ok := value.(*dictValue); return ok },
}

// executeBuiltinTypeCall handles one-argument type inspection and the current
// built-in constructors while preserving ordinary call stack cleanup.
func executeBuiltinTypeCall(
	caller *frame,
	instruction int,
	base int,
	class *builtinTypeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if class == cellType {
		if len(arguments) > 1 || keywordCount(keywords) != 0 {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "cell expected at most 1 argument",
			)}, nil
		}
		var value Value
		if len(arguments) == 1 {
			value = arguments[0]
		}
		return finishBuiltinTypeCall(caller, instruction, base, &cellValue{value: value})
	}
	if class == opaqueBuiltinTypes.function {
		if len(arguments) < 2 {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "function expected at least 2 arguments",
			)}, nil
		}
		code, codeOK := arguments[0].(*codeValue)
		globals, globalsException := mappingNamespace(arguments[1])
		if !codeOK || globalsException != nil {
			if globalsException != nil {
				return instructionOutcome{kind: raised, exception: globalsException}, nil
			}
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "function() argument 1 must be code",
			)}, nil
		}
		function := &functionValue{code: code.code, globals: globals}
		var closure Value = None
		var argdefs Value = None
		var kwdefaults Value = None
		if len(arguments) >= 4 {
			argdefs = arguments[3]
		}
		if len(arguments) >= 5 {
			closure = arguments[4]
		}
		if keywords != nil {
			for _, entry := range keywords.entries {
				name, ok := entry.key.(*stringValue)
				if !ok {
					continue
				}
				switch name.value {
				case "closure":
					closure = entry.value
				case "argdefs":
					argdefs = entry.value
				case "kwdefaults":
					kwdefaults = entry.value
				}
			}
		}
		if defaults, ok := argdefs.(*tupleValue); ok {
			function.defaults = append([]Value(nil), defaults.elements...)
		}
		if defaults, ok := kwdefaults.(*dictValue); ok {
			function.keywordDefaults = make(map[string]Value)
			for _, entry := range defaults.entries {
				if name, ok := entry.key.(*stringValue); ok {
					function.keywordDefaults[name.value] = entry.value
				}
			}
		}
		if cells, ok := closure.(*tupleValue); ok {
			for _, value := range cells.elements {
				if cell, ok := value.(*cellValue); ok {
					function.closure = append(function.closure, cell)
				}
			}
		}
		return finishBuiltinTypeCall(caller, instruction, base, function)
	}
	if class == opaqueBuiltinTypes.method {
		if len(arguments) != 2 {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "method expected 2 arguments",
			)}, nil
		}
		return finishBuiltinTypeCall(caller, instruction, base, &genericBoundMethodValue{
			callable: arguments[0], self: arguments[1],
		})
	}
	if class.name == "int" {
		if len(arguments) == 1 && (keywords == nil || len(keywords.entries) == 0) {
			if instance, ok := arguments[0].(*instanceValue); ok {
				method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__int__")
				if err != nil {
					return instructionOutcome{}, err
				}
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				if found {
					value, exception, err := callValueSynchronously(caller, method, nil)
					if err != nil {
						return instructionOutcome{}, err
					}
					if exception != nil {
						return instructionOutcome{kind: raised, exception: exception}, nil
					}
					integer, valid := integerOperand(value)
					if !valid {
						return instructionOutcome{kind: raised, exception: newException(
							"TypeError", "__int__ returned non-int",
						)}, nil
					}
					return finishBuiltinTypeCall(caller, instruction, base, &intValue{value: integer})
				}
			}
		}
		result, exception := constructInt(arguments, keywords)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return finishBuiltinTypeCall(caller, instruction, base, result)
	}
	if class.name == "dict" {
		result, exception, err := constructBuiltinDict(caller.runtime, arguments)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if keywords != nil {
			for _, entry := range keywords.entries {
				if exception := result.(*dictValue).set(entry.key, entry.value); exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
			}
		}
		return finishBuiltinTypeCall(caller, instruction, base, result)
	}
	if class == partialType {
		if len(arguments) == 0 {
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "type 'partial' takes at least one argument",
			)}, nil
		}
		value := &partialValue{
			callable:  arguments[0],
			arguments: append([]Value(nil), arguments[1:]...),
			keywords:  keywords,
		}
		return finishBuiltinTypeCall(caller, instruction, base, value)
	}
	if keywords != nil && len(keywords.entries) != 0 {
		return instructionOutcome{kind: raised, exception: newException(
			"TypeError", class.name+"() takes no keyword arguments",
		)}, nil
	}
	if class.name == "type" && len(arguments) == 1 {
		result := runtimeTypeOf(arguments[0])
		return finishBuiltinTypeCall(caller, instruction, base, result)
	}
	if class.name == "type" && len(arguments) == 3 {
		result, exception := constructDynamicType(arguments)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return finishBuiltinTypeCall(caller, instruction, base, result)
	}
	if class.name == "str" && len(arguments) == 1 {
		result, exception, err := pythonString(caller, arguments[0])
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return finishBuiltinTypeCall(caller, instruction, base, result)
	}
	if class.name == "bool" && len(arguments) == 1 {
		truth, exception, err := truthValueForFrame(caller, arguments[0])
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		return finishBuiltinTypeCall(caller, instruction, base, pythonBool(truth))
	}
	if class.name == "float" && len(arguments) == 1 {
		if instance, ok := arguments[0].(*instanceValue); ok {
			method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__float__")
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			if found {
				value, exception, err := callValueSynchronously(caller, method, nil)
				if err != nil {
					return instructionOutcome{}, err
				}
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				if _, ok := value.(*floatValue); !ok {
					return instructionOutcome{kind: raised, exception: newException(
						"TypeError", "__float__ returned non-float",
					)}, nil
				}
				return finishBuiltinTypeCall(caller, instruction, base, value)
			}
		}
	}
	if class.name == "complex" && len(arguments) <= 1 {
		if len(arguments) == 0 {
			return finishBuiltinTypeCall(caller, instruction, base, &complexValue{})
		}
		if instance, ok := arguments[0].(*instanceValue); ok {
			method, found, exception, err := lookupBoundSpecialMethod(caller, instance, "__complex__")
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			if found {
				value, exception, err := callValueSynchronously(caller, method, nil)
				if err != nil {
					return instructionOutcome{}, err
				}
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				if _, ok := value.(*complexValue); !ok {
					return instructionOutcome{kind: raised, exception: newException(
						"TypeError", "__complex__ returned non-complex",
					)}, nil
				}
				return finishBuiltinTypeCall(caller, instruction, base, value)
			}
		}
		if number, ok := numericComplex(arguments[0]); ok {
			return finishBuiltinTypeCall(caller, instruction, base, &complexValue{
				real: real(number), imaginary: imag(number),
			})
		}
	}
	result, exception, err := constructBuiltinValue(caller.runtime, class.name, arguments)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if result == nil {
		return instructionOutcome{kind: raised, exception: newException(
			"TypeError", class.name+"() is not implemented for these arguments",
		)}, nil
	}
	return finishBuiltinTypeCall(caller, instruction, base, result)
}

// constructInt implements integer conversion with positional or keyword bases.
func constructInt(arguments []Value, keywords *dictValue) (Value, *Exception) {
	if len(arguments) == 0 {
		return newInt64(0), nil
	}
	if len(arguments) > 2 {
		return nil, newException("TypeError", "int() takes at most 2 arguments")
	}
	base := int64(10)
	baseSupplied := len(arguments) == 2
	if baseSupplied {
		integer, ok := integerOperand(arguments[1])
		if !ok || !integer.IsInt64() {
			return nil, newException("TypeError", "int() base must be an integer")
		}
		base = integer.Int64()
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok || name.value != "base" || baseSupplied {
				return nil, newException("TypeError", "int() got an unexpected keyword argument")
			}
			integer, ok := integerOperand(entry.value)
			if !ok || !integer.IsInt64() {
				return nil, newException("TypeError", "int() base must be an integer")
			}
			base, baseSupplied = integer.Int64(), true
		}
	}
	if integer, ok := integerOperand(arguments[0]); ok && !baseSupplied {
		return &intValue{value: integer}, nil
	}
	if number, ok := arguments[0].(*floatValue); ok && !baseSupplied {
		if math.IsInf(number.value, 0) {
			return nil, newException("OverflowError", "cannot convert float infinity to integer")
		}
		if math.IsNaN(number.value) {
			return nil, newException("ValueError", "cannot convert float NaN to integer")
		}
		integer := new(big.Int)
		integer.SetInt64(int64(number.value))
		return &intValue{value: *integer}, nil
	}
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "int() argument must be a string or a number")
	}
	if base != 0 && (base < 2 || base > 36) {
		return nil, newException("ValueError", "int() base must be >= 2 and <= 36, or 0")
	}
	var integer big.Int
	if _, valid := integer.SetString(strings.TrimSpace(text.value), int(base)); !valid {
		return nil, newException("ValueError", "invalid literal for int()")
	}
	return &intValue{value: integer}, nil
}

// pythonString applies the exception and instance __str__ protocols before
// falling back to the stable representation of an immutable built-in value.
func pythonString(caller *frame, value Value) (*stringValue, *Exception, error) {
	switch value := value.(type) {
	case *stringValue:
		return value, nil, nil
	case *Exception:
		return &stringValue{value: value.message}, nil, nil
	case *instanceValue:
		method, found := value.class.lookup("__str__")
		if found {
			if native, ok := method.(*nativeFunctionValue); ok && strings.HasSuffix(native.name, ".__str__") {
				result, exception, err := builtinRepr(caller, []Value{value})
				if err != nil || exception != nil {
					return nil, exception, err
				}
				return result.(*stringValue), nil, nil
			}
			if resolved, descriptor, exception, err := resolvePythonDescriptor(
				caller, method, value, value.class,
			); descriptor {
				if err != nil || exception != nil {
					return nil, exception, err
				}
				method = resolved
			}
			result, exception, err := callValueSynchronously(
				caller, bindCallable(method, value), nil,
			)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			text, ok := result.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "__str__ returned non-string"), nil
			}
			return text, nil, nil
		}
	}
	return &stringValue{value: value.Repr()}, nil, nil
}

// constructDynamicType validates type(name, bases, namespace) and computes its C3 MRO.
func constructDynamicType(arguments []Value) (Value, *Exception) {
	name, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "type() argument 1 must be str")
	}
	bases, ok := arguments[1].(*tupleValue)
	if !ok {
		return nil, newException("TypeError", "type() argument 2 must be tuple")
	}
	namespace := newNamespace()
	switch mapping := arguments[2].(type) {
	case *dictValue:
		for _, entry := range mapping.entries {
			key, stringKey := entry.key.(*stringValue)
			if !stringKey {
				return nil, newException("TypeError", "type() dictionary keys must be strings")
			}
			namespace.values[key.value] = entry.value
		}
	case *namespaceValue:
		namespace = mapping.namespace
	default:
		return nil, newException("TypeError", "type() argument 3 must be dict")
	}
	class := &typeValue{
		name: name.value, qualifiedName: name.value, namespace: namespace,
	}
	if module, found := namespace.get("__module__"); found {
		if moduleName, stringModule := module.(*stringValue); stringModule {
			class.module = moduleName.value
		}
	}
	for _, base := range bases.elements {
		if alias, ok := base.(*genericAliasValue); ok {
			base = alias.origin
		}
		switch base := base.(type) {
		case *typeValue:
			class.bases = append(class.bases, base)
		case *builtinTypeValue:
			if base.name != "object" && class.builtinBase != nil {
				return nil, newException("TypeError", "multiple built-in bases are not supported")
			}
			if base.name != "object" {
				class.builtinBase = base
			}
		default:
			return nil, newException("TypeError", "type() bases must be types")
		}
	}
	var valid bool
	class.mro, valid = calculateMRO(class, class.bases)
	if !valid {
		return nil, newException("TypeError", "cannot create a consistent method resolution order")
	}
	initializeTestCaseSubclassState(class)
	return class, nil
}

func finishBuiltinTypeCall(
	caller *frame,
	instruction int,
	base int,
	result Value,
) (instructionOutcome, error) {
	for index := base; index < len(caller.stack); index++ {
		caller.stack[index] = nil
	}
	caller.stack = caller.stack[:base]
	return pushOutcome(caller, instruction, result)
}

// constructBuiltinValue dispatches supported scalar and collection type calls
// and returns nil when a known marker has no constructor implementation yet.
func constructBuiltinValue(runtime *Runtime, name string, arguments []Value) (Value, *Exception, error) {
	if name == "range" {
		value, exception := constructRange(arguments)
		return value, exception, nil
	}
	if name == "slice" {
		value, exception := constructSlice(arguments)
		return value, exception, nil
	}
	if name == "module" {
		if len(arguments) < 1 || len(arguments) > 2 {
			return nil, newException("TypeError", "module() takes one or two arguments"), nil
		}
		moduleName, ok := arguments[0].(*stringValue)
		if !ok {
			return nil, newException("TypeError", "module() name must be str"), nil
		}
		globals := newNamespace()
		globals.values["__name__"] = moduleName
		globals.values["__package__"] = None
		globals.values["__loader__"] = None
		globals.values["__spec__"] = None
		globals.values["__doc__"] = None
		if len(arguments) == 2 {
			globals.values["__doc__"] = arguments[1]
		}
		return &Module{name: moduleName.value, globals: globals}, nil, nil
	}
	if len(arguments) > 1 {
		return nil, newException("TypeError", name+"() expected at most 1 argument"), nil
	}
	switch name {
	case "object":
		if len(arguments) == 0 {
			return &objectValue{}, nil, nil
		}
	case "list", "tuple", "set", "frozenset":
		return constructBuiltinCollection(runtime, name, arguments)
	case "dict":
		return constructBuiltinDict(runtime, arguments)
	case "mappingproxy":
		if len(arguments) == 1 {
			if dictionary, ok := arguments[0].(*dictValue); ok {
				return dictionary, nil, nil
			}
			if namespace, ok := arguments[0].(*namespaceValue); ok {
				return namespace, nil, nil
			}
			if instance, ok := arguments[0].(*instanceValue); ok && instance.mapping != nil {
				return instance.mapping, nil, nil
			}
		}
	case "bytes":
		if len(arguments) == 0 {
			return &bytesValue{}, nil, nil
		}
		if value, ok := arguments[0].(*bytesValue); ok {
			return value, nil, nil
		}
	case "bytearray":
		if len(arguments) == 0 {
			return &bytearrayValue{}, nil, nil
		}
		if value, ok := arguments[0].(*bytesValue); ok {
			return &bytearrayValue{value: value.value}, nil, nil
		}
	case "StringIO":
		return newStringIO(nil, arguments)
	case "BytesIO":
		return newBytesIO(nil, arguments)
	case "bool":
		if len(arguments) == 0 {
			return falseSingleton, nil, nil
		}
		return pythonBool(truthValue(arguments[0])), nil, nil
	case "str":
		if len(arguments) == 0 {
			return &stringValue{}, nil, nil
		}
		if value, ok := arguments[0].(*stringValue); ok {
			return value, nil, nil
		}
		if exception, ok := arguments[0].(*Exception); ok {
			return &stringValue{value: exception.message}, nil, nil
		}
		return &stringValue{value: arguments[0].Repr()}, nil, nil
	case "int":
		if len(arguments) == 0 {
			return newInt64(0), nil, nil
		}
		if integer, ok := integerOperand(arguments[0]); ok {
			return &intValue{value: integer}, nil, nil
		}
	case "float":
		if len(arguments) == 0 {
			return &floatValue{}, nil, nil
		}
		if value, ok := numericFloat(arguments[0]); ok {
			return &floatValue{value: value}, nil, nil
		}
		if text, ok := arguments[0].(*stringValue); ok {
			normalized := strings.TrimSpace(text.value)
			switch strings.ToLower(normalized) {
			case "inf", "+inf", "infinity", "+infinity":
				return &floatValue{value: math.Inf(1)}, nil, nil
			case "-inf", "-infinity":
				return &floatValue{value: math.Inf(-1)}, nil, nil
			case "nan", "+nan", "-nan":
				return &floatValue{value: math.NaN()}, nil, nil
			}
			value, err := strconv.ParseFloat(normalized, 64)
			if err != nil {
				return nil, newException("ValueError", "could not convert string to float"), nil
			}
			return &floatValue{value: value}, nil, nil
		}
	}
	return nil, nil, nil
}

// constructBuiltinDict copies a mapping or consumes an iterable of key-value pairs.
func constructBuiltinDict(runtime *Runtime, arguments []Value) (Value, *Exception, error) {
	dictionary := &dictValue{}
	if len(arguments) == 0 {
		return dictionary, nil, nil
	}
	if source, ok := arguments[0].(*dictValue); ok {
		dictionary.entries = append([]dictEntry(nil), source.entries...)
		return dictionary, nil, nil
	}
	if source, ok := arguments[0].(*namespaceValue); ok {
		for name, value := range source.namespace.values {
			dictionary.entries = append(dictionary.entries, dictEntry{
				key: &stringValue{value: name}, value: value,
			})
		}
		return dictionary, nil, nil
	}
	if source, ok := arguments[0].(*instanceValue); ok && source.mapping != nil {
		dictionary.entries = append([]dictEntry(nil), source.mapping.entries...)
		return dictionary, nil, nil
	}
	iterator, exception, err := newIteratorForRuntime(runtime, arguments[0])
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if iterator == nil {
		return nil, newException("TypeError", "cannot convert dictionary update sequence element"), nil
	}
	for {
		item, available, exception, err := nextNativeIterator(runtime, iterator)
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if !available {
			return dictionary, nil, nil
		}
		var pair []Value
		switch item := item.(type) {
		case *tupleValue:
			pair = item.elements
		case *listValue:
			pair = item.elements
		}
		if len(pair) != 2 {
			return nil, newException("ValueError", "dictionary update sequence element has length other than 2"), nil
		}
		if exception := dictionary.set(pair[0], pair[1]); exception != nil {
			return nil, exception, nil
		}
	}
}

type objectValue struct{ marker byte }

func (*objectValue) TypeName() string { return "object" }
func (*objectValue) Repr() string     { return "<object object>" }
func (*objectValue) isValue()         {}

var _ Value = (*objectValue)(nil)
var _ Value = (*notImplementedValue)(nil)

// constructBuiltinCollection exhausts one iterable and then builds the
// selected mutable or immutable collection representation.
func constructBuiltinCollection(runtime *Runtime, name string, arguments []Value) (Value, *Exception, error) {
	var elements []Value
	if len(arguments) == 1 {
		iterator, exception, err := newIteratorForRuntime(runtime, arguments[0])
		if err != nil || exception != nil {
			return nil, exception, err
		}
		if iterator == nil {
			return nil, newException("TypeError", "'"+arguments[0].TypeName()+"' object is not iterable"), nil
		}
		for {
			value, available, exception, err := nextNativeIterator(runtime, iterator)
			if err != nil || exception != nil {
				return nil, exception, err
			}
			if !available {
				break
			}
			elements = append(elements, value)
		}
	}
	switch name {
	case "list":
		return &listValue{elements: elements}, nil, nil
	case "tuple":
		return &tupleValue{elements: elements}, nil, nil
	case "frozenset":
		set := &setValue{}
		for _, value := range elements {
			if exception := set.add(value); exception != nil {
				return nil, exception, nil
			}
		}
		return &frozenSetValue{entries: set.entries}, nil, nil
	default:
		set := &setValue{}
		for _, value := range elements {
			if exception := set.add(value); exception != nil {
				return nil, exception, nil
			}
		}
		return set, nil, nil
	}
}

// runtimeTypeOf maps each current runtime representation to its stable Python
// class value, preserving user classes and descriptor subclasses.
func runtimeTypeOf(value Value) Value {
	switch value := value.(type) {
	case *instanceValue:
		return value.class
	case *typeValue:
		if value.metaclass != nil {
			return value.metaclass
		}
		return builtinTypeNamed("type")
	case *builtinTypeValue, *exceptionTypeValue:
		return builtinTypeNamed("type")
	case *descriptorValue:
		return value.class
	case *typeMetadataDescriptorValue:
		return opaqueBuiltinTypes.getSet
	case *memberDescriptorValue:
		return opaqueBuiltinTypes.member
	case *cellValue:
		return cellType
	case *futureValue:
		return futureType
	case *Exception:
		if value.userClass != nil {
			return value.userClass
		}
		return value.class
	case *genericAliasValue:
		return genericAliasType
	case *unionValue:
		return unionType
	case *templateValue:
		return templateType
	case *functionValue:
		return opaqueBuiltinTypes.function
	case *boundMethodValue, *genericBoundMethodValue:
		return opaqueBuiltinTypes.method
	case *ellipsisValue:
		return opaqueBuiltinTypes.ellipsis
	case *coroutineValue:
		return opaqueBuiltinTypes.coroutine
	case *asyncGeneratorValue:
		return opaqueBuiltinTypes.asyncGen
	case *partialValue:
		return partialType
	case *generatorValue:
		return opaqueBuiltinTypes.generator
	case valueIterator:
		return opaqueBuiltinTypes.iterator
	case *dictViewValue:
		return dictViewTypes[value.kind]
	case *bytearrayValue:
		return builtinTypeNamed("bytearray")
	}
	for _, class := range builtinTypes {
		if class.name != "object" && class.matches(value) {
			return class
		}
	}
	return builtinTypeNamed("object")
}

func builtinTypeNamed(name string) *builtinTypeValue {
	for _, class := range builtinTypes {
		if class.name == name {
			return class
		}
	}
	panic("missing built-in type: " + name)
}

// valueIsInstance matches built-in markers, user classes, exception classes,
// and recursive tuples while retaining invalid class information as TypeError.
func valueIsInstance(value Value, classInfo Value) (bool, *Exception) {
	switch classInfo := classInfo.(type) {
	case *builtinTypeValue:
		if instance, ok := value.(*instanceValue); ok &&
			instance.class.inheritedBuiltinBase() == classInfo {
			return true, nil
		}
		return classInfo.matches(value), nil
	case *typeValue:
		switch value := value.(type) {
		case *instanceValue:
			if specified, found := value.attributes.get("_spec_class"); found && specified != None {
				if specifiedClass, ok := specified.(*typeValue); ok && specifiedClass.isSubclassOf(classInfo) {
					return true, nil
				}
			}
			return value.class.isSubclassOf(classInfo), nil
		case *typeValue:
			return value.metaclass != nil && value.metaclass.isSubclassOf(classInfo), nil
		default:
			if runtimeClass, ok := runtimeTypeOf(value).(*typeValue); ok {
				return runtimeClass.isSubclassOf(classInfo), nil
			}
			return false, nil
		}
	case *exceptionTypeValue:
		exception, ok := value.(*Exception)
		if !ok {
			return false, nil
		}
		if exception.userClass != nil {
			base := exception.userClass.builtinExceptionBase()
			return base != nil && base.isSubclassOf(classInfo), nil
		}
		return exception.class.isSubclassOf(classInfo), nil
	case *tupleValue:
		for _, candidate := range classInfo.elements {
			matches, exception := valueIsInstance(value, candidate)
			if exception != nil || matches {
				return matches, exception
			}
		}
		return false, nil
	default:
		return false, newException(
			"TypeError",
			"isinstance() arg 2 must be a type, a tuple of types, or a union",
		)
	}
}

var _ Value = (*builtinTypeValue)(nil)
