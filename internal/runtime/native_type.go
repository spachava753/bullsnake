package runtime

// nativeTypeValue is the stable class identity for a runtime value implemented
// in Go. User classes and exception classes retain their existing class values.
type nativeTypeValue struct {
	name     string
	qualname string
	module   string
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
	typeNativeType            = builtinNativeType("type")
	noneNativeType            = builtinNativeType("NoneType")
	boolNativeType            = builtinNativeType("bool")
	intNativeType             = builtinNativeType("int")
	floatNativeType           = builtinNativeType("float")
	complexNativeType         = builtinNativeType("complex")
	stringNativeType          = builtinNativeType("str")
	bytesNativeType           = builtinNativeType("bytes")
	tupleNativeType           = builtinNativeType("tuple")
	listNativeType            = builtinNativeType("list")
	dictNativeType            = builtinNativeType("dict")
	setNativeType             = builtinNativeType("set")
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
	boolNativeType,
	intNativeType,
	stringNativeType,
}

var nativeTypesByRuntimeName = map[string]*nativeTypeValue{
	"NoneType":                         noneNativeType,
	"bool":                             boolNativeType,
	"int":                              intNativeType,
	"float":                            floatNativeType,
	"complex":                          complexNativeType,
	"str":                              stringNativeType,
	"bytes":                            bytesNativeType,
	"tuple":                            tupleNativeType,
	"list":                             listNativeType,
	"dict":                             dictNativeType,
	"set":                              setNativeType,
	"slice":                            sliceNativeType,
	"ellipsis":                         ellipsisNativeType,
	"NotImplementedType":               notImplementedNativeType,
	"function":                         functionNativeType,
	"builtin_function_or_method":       builtinFunctionNativeType,
	"method-wrapper":                   builtinFunctionNativeType,
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
			discardCallSegment(caller, base)
			return raiseOutcome(newException(
				"NotImplementedError",
				"three-argument type() is not supported",
			)), nil
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
	}
	discardCallSegment(caller, base)
	return raiseOutcome(newException(
		"NotImplementedError",
		class.name+"() constructor is not supported",
	)), nil
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

func executeNativeTypeAttributeLoad(
	frame *frame,
	instruction int,
	class *nativeTypeValue,
	name string,
) (instructionOutcome, error) {
	var value Value
	switch name {
	case "__name__":
		value = &stringValue{value: class.name}
	case "__qualname__":
		value = &stringValue{value: class.qualname}
	case "__module__":
		value = &stringValue{value: class.module}
	default:
		return raiseOutcome(newException(
			"AttributeError",
			"type object '"+class.name+"' has no attribute '"+name+"'",
		)), nil
	}
	return pushOutcome(frame, instruction, value)
}

var _ Value = (*nativeTypeValue)(nil)
