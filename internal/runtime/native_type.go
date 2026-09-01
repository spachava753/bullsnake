package runtime

// nativeTypeValue is the stable class identity for a runtime value implemented
// in Go. User classes and exception classes retain their existing class values.
type nativeTypeValue struct {
	name     string
	qualname string
	module   string
	base     *nativeTypeValue
}

func (*nativeTypeValue) TypeName() string { return "type" }
func (class *nativeTypeValue) Repr() string {
	name := class.qualname
	if name == "" {
		name = class.name
	}
	if class.module == "" || class.module == "builtins" {
		return "<class '" + name + "'>"
	}
	return "<class '" + class.module + "." + name + "'>"
}
func (*nativeTypeValue) isValue() {}

func nativeType(module, name string) *nativeTypeValue {
	return &nativeTypeValue{name: name, qualname: name, module: module}
}

func builtinNativeType(name string) *nativeTypeValue {
	return nativeType("builtins", name)
}

var (
	typeNativeType   = builtinNativeType("type")
	objectNativeType = builtinNativeType("object")
	noneNativeType   = builtinNativeType("NoneType")
	intNativeType    = builtinNativeType("int")
	boolNativeType   = &nativeTypeValue{
		name: "bool", qualname: "bool", module: "builtins", base: intNativeType,
	}
	floatNativeType           = builtinNativeType("float")
	complexNativeType         = builtinNativeType("complex")
	stringNativeType          = builtinNativeType("str")
	bytesNativeType           = builtinNativeType("bytes")
	rangeNativeType           = builtinNativeType("range")
	enumerateNativeType       = builtinNativeType("enumerate")
	mapNativeType             = builtinNativeType("map")
	filterNativeType          = builtinNativeType("filter")
	zipNativeType             = builtinNativeType("zip")
	tupleNativeType           = builtinNativeType("tuple")
	listNativeType            = builtinNativeType("list")
	dictNativeType            = builtinNativeType("dict")
	setNativeType             = builtinNativeType("set")
	frozenSetNativeType       = builtinNativeType("frozenset")
	sliceNativeType           = builtinNativeType("slice")
	ellipsisNativeType        = builtinNativeType("ellipsis")
	notImplementedNativeType  = builtinNativeType("NotImplementedType")
	functionNativeType        = builtinNativeType("function")
	builtinFunctionNativeType = builtinNativeType("builtin_function_or_method")
	methodNativeType          = builtinNativeType("method")
	moduleNativeType          = builtinNativeType("module")
	propertyNativeType        = builtinNativeType("property")
	classMethodNativeType     = builtinNativeType("classmethod")
	staticMethodNativeType    = builtinNativeType("staticmethod")
	superNativeType           = builtinNativeType("super")
	generatorNativeType       = builtinNativeType("generator")
	coroutineNativeType       = builtinNativeType("coroutine")
	asyncGeneratorNativeType  = builtinNativeType("async_generator")
)

var builtinNativeTypes = []*nativeTypeValue{
	typeNativeType,
	objectNativeType,
	boolNativeType,
	intNativeType,
	stringNativeType,
	rangeNativeType,
	enumerateNativeType,
	mapNativeType,
	filterNativeType,
	zipNativeType,
	tupleNativeType,
	listNativeType,
	setNativeType,
	frozenSetNativeType,
	dictNativeType,
}

var nativeTypesByRuntimeName = map[string]*nativeTypeValue{
	"object":                           objectNativeType,
	"NoneType":                         noneNativeType,
	"bool":                             boolNativeType,
	"int":                              intNativeType,
	"float":                            floatNativeType,
	"complex":                          complexNativeType,
	"str":                              stringNativeType,
	"bytes":                            bytesNativeType,
	"range":                            rangeNativeType,
	"enumerate":                        enumerateNativeType,
	"map":                              mapNativeType,
	"filter":                           filterNativeType,
	"zip":                              zipNativeType,
	"tuple":                            tupleNativeType,
	"list":                             listNativeType,
	"dict":                             dictNativeType,
	"dict_keys":                        builtinNativeType("dict_keys"),
	"dict_items":                       builtinNativeType("dict_items"),
	"dict_values":                      builtinNativeType("dict_values"),
	"dict_itemiterator":                builtinNativeType("dict_itemiterator"),
	"dict_valueiterator":               builtinNativeType("dict_valueiterator"),
	"set":                              setNativeType,
	"frozenset":                        frozenSetNativeType,
	"slice":                            sliceNativeType,
	"ellipsis":                         ellipsisNativeType,
	"NotImplementedType":               notImplementedNativeType,
	"function":                         functionNativeType,
	"builtin_function_or_method":       builtinFunctionNativeType,
	"method-wrapper":                   builtinFunctionNativeType,
	"method_descriptor":                builtinNativeType("method_descriptor"),
	"method":                           methodNativeType,
	"module":                           moduleNativeType,
	"property":                         propertyNativeType,
	"classmethod":                      classMethodNativeType,
	"staticmethod":                     staticMethodNativeType,
	"super":                            superNativeType,
	"generator":                        generatorNativeType,
	"coroutine":                        coroutineNativeType,
	"async_generator":                  asyncGeneratorNativeType,
	"async_generator_asend":            builtinNativeType("async_generator_asend"),
	"async_generator_athrow":           builtinNativeType("async_generator_athrow"),
	"cell":                             builtinNativeType("cell"),
	"tuple_iterator":                   builtinNativeType("tuple_iterator"),
	"list_iterator":                    builtinNativeType("list_iterator"),
	"str_iterator":                     builtinNativeType("str_iterator"),
	"bytes_iterator":                   builtinNativeType("bytes_iterator"),
	"range_iterator":                   builtinNativeType("range_iterator"),
	"reversed":                         builtinNativeType("reversed"),
	"list_reverseiterator":             builtinNativeType("list_reverseiterator"),
	"dict_keyiterator":                 builtinNativeType("dict_keyiterator"),
	"set_iterator":                     builtinNativeType("set_iterator"),
	"iterator":                         builtinNativeType("iterator"),
	"_Feature":                         nativeType("__future__", "_Feature"),
	"NoDefaultType":                    nativeType("typing", "NoDefaultType"),
	"typing.TypeVar":                   nativeType("typing", "TypeVar"),
	"typing.TypeVarTuple":              nativeType("typing", "TypeVarTuple"),
	"typing.ParamSpec":                 nativeType("typing", "ParamSpec"),
	"typing.ParamSpecArgs":             nativeType("typing", "ParamSpecArgs"),
	"typing.ParamSpecKwargs":           nativeType("typing", "ParamSpecKwargs"),
	"typing.TypeAliasType":             nativeType("typing", "TypeAliasType"),
	"functools.KeyWrapper":             nativeType("functools", "KeyWrapper"),
	"string.templatelib.Template":      nativeType("string.templatelib", "Template"),
	"string.templatelib.Interpolation": nativeType("string.templatelib", "Interpolation"),
	"string.templatelib.TemplateIter":  nativeType("string.templatelib", "TemplateIter"),
}

// executeNativeTypeCall preserves the existing scalar constructors and handles
// the one-argument introspection form of type.
func executeNativeTypeCall(
	caller *frame,
	instruction int,
	base int,
	class *nativeTypeValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if class == typeNativeType {
		if keywords != nil && len(keywords.entries) != 0 {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"TypeError",
				"type() takes 1 or 3 arguments",
			)), nil
		}
		if len(arguments) == 3 {
			return executeDynamicTypeCall(
				caller,
				instruction,
				base,
				arguments,
			)
		}
		if len(arguments) != 1 {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"TypeError",
				"type() takes 1 or 3 arguments",
			)), nil
		}
		result, exception := typeOf(arguments[0])
		discardCallSegment(caller, base)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, result)
	}
	switch class {
	case objectNativeType:
		return executeObjectTypeCall(caller, instruction, base, arguments, keywords)
	case boolNativeType:
		return executeBuiltinBool(caller, instruction, base, arguments, keywords)
	case intNativeType:
		result, exception := builtinInt(arguments, keywords)
		discardCallSegment(caller, base)
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(caller, instruction, result)
	case stringNativeType:
		return executeBuiltinStr(caller, instruction, base, arguments, keywords)
	case rangeNativeType:
		return executeRangeTypeCall(caller, instruction, base, arguments, keywords)
	case enumerateNativeType:
		return executeEnumerateTypeCall(caller, instruction, base, arguments, keywords)
	case mapNativeType:
		return executeMapTypeCall(caller, instruction, base, arguments, keywords)
	case filterNativeType:
		return executeFilterTypeCall(caller, instruction, base, arguments, keywords)
	case zipNativeType:
		return executeZipTypeCall(caller, instruction, base, arguments, keywords)
	case tupleNativeType, listNativeType, setNativeType, frozenSetNativeType,
		dictNativeType:
		return executeCollectionTypeCall(
			caller,
			instruction,
			base,
			class,
			arguments,
			keywords,
		)
	}
	discardCallSegment(caller, base)
	return raiseOutcome(newException(
		"NotImplementedError",
		class.name+"() constructor is not supported",
	)), nil
}

// executeDynamicTypeCall copies one validated namespace into the ordinary class
// builder so dynamic and statement classes share C3 and descriptor finalization.
func executeDynamicTypeCall(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
) (instructionOutcome, error) {
	name, ok := arguments[0].(*stringValue)
	if !ok {
		invalidType := arguments[0].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"type.__new__() argument 1 must be str, not "+invalidType,
		)), nil
	}
	baseTuple, ok := arguments[1].(*tupleValue)
	if !ok {
		invalidType := arguments[1].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"type.__new__() argument 2 must be tuple, not "+invalidType,
		)), nil
	}
	dictionary, ok := arguments[2].(*dictValue)
	if !ok {
		invalidType := arguments[2].TypeName()
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"type.__new__() argument 3 must be dict, not "+invalidType,
		)), nil
	}
	bases, exceptionBase, objectBase, exception := resolveClassBases(baseTuple.elements)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}

	namespace := newNamespace()
	build := &classBuild{
		name:              name.value,
		qualifiedName:     name.value,
		namespace:         namespace,
		bases:             bases,
		objectBase:        objectBase,
		exceptionBase:     exceptionBase,
		namespacePosition: make(map[string]int),
	}
	for _, entry := range dictionary.entries {
		key, stringKey := entry.key.(*stringValue)
		if !stringKey {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"TypeError",
				"type namespace keys must be strings",
			)), nil
		}
		namespace.values[key.value] = entry.value
		build.recordStore(key.value)
	}

	if moduleValue, found := namespace.get("__module__"); found {
		if module, stringModule := moduleValue.(*stringValue); stringModule {
			build.module = module.value
		}
	} else {
		if callerModule, found := caller.globals.get("__name__"); found {
			if module, stringModule := callerModule.(*stringValue); stringModule {
				build.module = module.value
			}
		}
		namespace.values["__module__"] = &stringValue{value: build.module}
		build.recordStore("__module__")
	}
	if qualifiedValue, found := namespace.get("__qualname__"); found {
		qualified, stringQualified := qualifiedValue.(*stringValue)
		if !stringQualified {
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"TypeError",
				"type __qualname__ must be a str, not "+qualifiedValue.TypeName(),
			)), nil
		}
		build.qualifiedName = qualified.value
	} else {
		namespace.values["__qualname__"] = &stringValue{value: name.value}
		build.recordStore("__qualname__")
	}

	result, exception := build.finish(None)
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, result)
}

// typeOf returns an existing user or exception class before consulting the
// immutable native type table for Go-backed values.
func typeOf(value Value) (Value, *Exception) {
	switch value := value.(type) {
	case *nativeTypeValue, *typeValue, *exceptionTypeValue:
		return typeNativeType, nil
	case *instanceValue:
		return value.class, nil
	case *Exception:
		if value.userClass != nil {
			return value.userClass, nil
		}
		return value.class, nil
	}
	class, found := nativeTypesByRuntimeName[value.TypeName()]
	if !found {
		return nil, newException(
			"NotImplementedError",
			"type identity for '"+value.TypeName()+"' is not supported",
		)
	}
	return class, nil
}

func executeExceptionTypeAttributeLoad(
	frame *frame,
	instruction int,
	class *exceptionTypeValue,
	name string,
) (instructionOutcome, error) {
	var value Value
	switch name {
	case "__name__", "__qualname__":
		value = &stringValue{value: class.name}
	case "__module__":
		value = &stringValue{value: "builtins"}
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"type object '"+class.name+"' has no attribute '"+name+"'",
		)), nil
	}
	return pushOutcome(frame, instruction, value)
}

// executeNativeTypeAttributeLoad returns immutable class metadata and the native
// method descriptors currently exposed by a built-in class.
func executeNativeTypeAttributeLoad(
	frame *frame,
	instruction int,
	class *nativeTypeValue,
	name string,
) (instructionOutcome, error) {
	if name == "__contains__" {
		switch class {
		case setNativeType:
			return pushOutcome(
				frame,
				instruction,
				&setContainsDescriptor{},
			)
		case frozenSetNativeType:
			return pushOutcome(
				frame,
				instruction,
				&setContainsDescriptor{frozen: true},
			)
		}
	}
	var value Value
	switch name {
	case "__name__":
		value = &stringValue{value: class.name}
	case "__qualname__":
		value = &stringValue{value: class.qualname}
	case "__module__":
		value = &stringValue{value: class.module}
	case "__base__":
		if class == objectNativeType {
			value = None
		} else if class.base != nil {
			value = class.base
		} else {
			value = objectNativeType
		}
	case "__bases__":
		if class == objectNativeType {
			value = &tupleValue{}
		} else if class.base != nil {
			value = &tupleValue{elements: []Value{class.base}}
		} else {
			value = &tupleValue{elements: []Value{objectNativeType}}
		}
	case "__mro__":
		elements := []Value{class}
		if class.base != nil {
			elements = append(elements, class.base)
		}
		if class != objectNativeType {
			elements = append(elements, objectNativeType)
		}
		value = &tupleValue{elements: elements}
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"type object '"+class.name+"' has no attribute '"+name+"'",
		)), nil
	}
	return pushOutcome(frame, instruction, value)
}

var _ Value = (*nativeTypeValue)(nil)
