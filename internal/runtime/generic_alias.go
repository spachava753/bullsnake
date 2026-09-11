package runtime

import "strings"

type genericAliasState struct {
	origin     Value
	args       *tupleValue
	parameters *tupleValue
}

// initializeGenericAliasClass installs read-only metadata and representation on
// one immutable class per runtime; instances retain their original argument tuple.
func initializeGenericAliasClass() *typeValue {
	class := newBuiltinClass("types", "GenericAlias", nil)
	class.genericAliasClass = true
	class.setAttribute("__new__", &builtinFunctionValue{name: "GenericAlias.__new__", frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		if exception := checkNativeArguments("GenericAlias.__new__", arguments, keywords, 3, 3); exception != nil {
			discardCallSegment(caller, base)
			return raiseOutcome(exception), nil
		}
		subclass, ok := arguments[0].(*typeValue)
		if !ok || !subclass.isSubclassOf(class) {
			discardCallSegment(caller, base)
			return raiseOutcome(newException("TypeError", "GenericAlias.__new__ requires a GenericAlias subtype")), nil
		}
		value := newGenericAlias(subclass, arguments[1], arguments[2])
		discardCallSegment(caller, base)
		return pushOutcome(caller, instruction, value)
	}})
	class.setAttribute("__parameters__", &propertyValue{doc: None, getter: nativeInstanceMethod(class, "__parameters__", executeGenericAliasParameters)})
	class.setAttribute("__hash__", nativeInstanceMethod(class, "__hash__", executeGenericAliasHash))
	for _, name := range []string{"__eq__", "__ne__"} {
		class.setAttribute(name, nativeInstanceMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			if exception := checkNativeArguments(name, arguments, keywords, 1, 1); exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeGenericAliasEquality(caller, instruction, self, arguments[0], name == "__ne__")
		}))
	}
	class.setAttribute("__call__", nativeInstanceMethod(class, "__call__", executeGenericAliasCall))
	for _, name := range []string{"__origin__", "__args__"} {
		class.setAttribute(name, &propertyValue{doc: None, getter: nativeInstanceMethod(class, name, func(caller *frame, instruction int, self *instanceValue, _ []Value, _ *dictValue) (instructionOutcome, error) {
			if self.alias == nil {
				return raiseOutcome(newException("TypeError", "uninitialized GenericAlias")), nil
			}
			if name == "__origin__" {
				return pushOutcome(caller, instruction, self.alias.origin)
			}
			return pushOutcome(caller, instruction, self.alias.args)
		})})
	}
	for _, name := range []string{"__repr__", "__mro_entries__"} {
		class.setAttribute(name, nativeInstanceMethod(class, name, func(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
			count := 0
			if name == "__mro_entries__" {
				count = 1
			}
			if exception := checkNativeArguments(name, arguments, keywords, count, count); exception != nil {
				return raiseOutcome(exception), nil
			}
			if self.alias == nil {
				return raiseOutcome(newException("TypeError", "uninitialized GenericAlias")), nil
			}
			if name == "__mro_entries__" {
				return pushOutcome(caller, instruction, &tupleValue{elements: []Value{self.alias.origin}})
			}
			return pushOutcome(caller, instruction, &stringValue{value: self.alias.repr()})
		}))
	}
	return class
}

func newGenericAlias(class *typeValue, origin, arguments Value) *instanceValue {
	args, ok := arguments.(*tupleValue)
	if !ok {
		args = &tupleValue{elements: []Value{arguments}}
	}
	return &instanceValue{class: class, attributes: newNamespace(), alias: &genericAliasState{origin: origin, args: args}}
}

func executeGenericAliasConstructor(caller *frame, instruction, base int, class *typeValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if !class.genericAliasClass {
		return executeGenericAliasSubclass(caller, instruction, base, class, arguments, keywords)
	}
	if exception := checkNativeArguments("GenericAlias", arguments, keywords, 2, 2); exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	value := newGenericAlias(class, arguments[0], arguments[1])
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, value)
}

func (alias *genericAliasState) repr() string {
	parts := make([]string, len(alias.args.elements))
	for index, arg := range alias.args.elements {
		parts[index] = genericAliasArgumentRepr(arg)
	}
	arguments := strings.Join(parts, ", ")
	if len(parts) == 0 {
		arguments = "()"
	}
	return genericAliasArgumentRepr(alias.origin) + "[" + arguments + "]"
}

// genericAliasArgumentRepr omits builtins qualification and renders nested
// aliases recursively, while ordinary values use the runtime's fixed repr rules.
func genericAliasArgumentRepr(value Value) string {
	switch value := value.(type) {
	case *nativeTypeValue:
		if value.module == "builtins" {
			return value.qualname
		}
		return value.module + "." + value.qualname
	case *typeValue:
		if value.module == "builtins" || value.module == "" {
			return value.qualifiedName
		}
		return value.module + "." + value.qualifiedName
	case *instanceValue:
		if value.alias != nil {
			return value.alias.repr()
		}
	case *ellipsisValue:
		return "..."
	}
	return value.Repr()
}

func hasNativeClassGetitem(class *nativeTypeValue) bool {
	switch class {
	case listNativeType, tupleNativeType, dictNativeType, setNativeType, frozenSetNativeType:
		return true
	}
	return false
}

func nativeClassGetitem(class *nativeTypeValue) Value {
	return &builtinFunctionValue{name: "__class_getitem__", frameCall: func(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
		if exception := checkNativeArguments("__class_getitem__", arguments, keywords, 2, 2); exception != nil {
			discardCallSegment(caller, base)
			return raiseOutcome(exception), nil
		}
		if arguments[0] != class {
			discardCallSegment(caller, base)
			return raiseOutcome(newException("TypeError", "descriptor '__class_getitem__' requires a '"+class.name+"' type")), nil
		}
		alias := newGenericAlias(caller.runtime.genericAliasClass, arguments[0], arguments[1])
		discardCallSegment(caller, base)
		return pushOutcome(caller, instruction, alias)
	}}
}
