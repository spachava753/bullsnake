package runtime

import "strings"

type genericAliasValue struct {
	origin Value
	args   *tupleValue
}

type unionValue struct {
	elements []Value
}

func (*unionValue) TypeName() string { return "types.UnionType" }
func (union *unionValue) Repr() string {
	parts := make([]string, len(union.elements))
	for index, element := range union.elements {
		parts[index] = element.Repr()
	}
	return strings.Join(parts, " | ")
}
func (*unionValue) isValue() {}
func (union *unionValue) attribute(name string) (Value, bool) {
	if name != "__args__" {
		return nil, false
	}
	return &tupleValue{elements: append([]Value(nil), union.elements...)}, true
}

var unionType = &builtinTypeValue{
	name:    "types.UnionType",
	matches: func(value Value) bool { _, ok := value.(*unionValue); return ok },
}

// unionOperands combines class and existing-union operands for the PEP 604 operator.
func unionOperands(left, right Value) (Value, bool) {
	if !isClassValue(left) {
		if _, ok := left.(*unionValue); !ok {
			return nil, false
		}
	}
	if !isClassValue(right) {
		if _, ok := right.(*unionValue); !ok {
			return nil, false
		}
	}
	elements := make([]Value, 0, 2)
	for _, value := range []Value{left, right} {
		if union, ok := value.(*unionValue); ok {
			elements = append(elements, union.elements...)
		} else {
			elements = append(elements, value)
		}
	}
	return &unionValue{elements: elements}, true
}

func (*genericAliasValue) TypeName() string { return "types.GenericAlias" }
func (alias *genericAliasValue) Repr() string {
	arguments := alias.args.Repr()
	if len(alias.args.elements) == 1 {
		arguments = alias.args.elements[0].Repr()
	}
	return alias.origin.Repr() + "[" + arguments + "]"
}
func (*genericAliasValue) isValue() {}
func (alias *genericAliasValue) attribute(name string) (Value, bool) {
	switch name {
	case "__origin__":
		return alias.origin, true
	case "__args__":
		return alias.args, true
	default:
		return nil, false
	}
}

func newGenericAlias(origin, arguments Value) *genericAliasValue {
	if tuple, ok := arguments.(*tupleValue); ok {
		return &genericAliasValue{origin: origin, args: tuple}
	}
	return &genericAliasValue{
		origin: origin,
		args:   &tupleValue{elements: []Value{arguments}},
	}
}

func constructGenericAlias(
	_ *typeValue,
	_ *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	if len(arguments) != 2 || keywordCount(keywords) != 0 {
		return nil, newException("TypeError", "GenericAlias() takes exactly 2 arguments"), nil
	}
	return newGenericAlias(arguments[0], arguments[1]), nil, nil
}

var genericAliasType = &typeValue{
	name:          "GenericAlias",
	qualifiedName: "GenericAlias",
	module:        "types",
	namespace:     newNamespace(),
	constructor:   constructGenericAlias,
}

// intrinsicTypeAttribute exposes stable class metadata independently from the
// user-defined namespace and materializes bases and MRO tuples on demand.
func intrinsicTypeAttribute(class *typeValue, name string) (Value, bool) {
	switch name {
	case "__name__":
		return &stringValue{value: class.name}, true
	case "__qualname__":
		return &stringValue{value: class.qualifiedName}, true
	case "__module__":
		return &stringValue{value: class.module}, true
	case "__doc__":
		if doc, found := class.namespace.get("__doc__"); found {
			return doc, true
		}
		return None, true
	case "__flags__":
		// Bullsnake has no CPython type flags. A zero value accurately reports
		// that its bootstrap classes have not been finalized as abstract types.
		return newInt64(0), true
	case "__dict__":
		return &namespaceValue{namespace: class.namespace}, true
	case "__bases__":
		bases := make([]Value, len(class.bases))
		for index, base := range class.bases {
			bases[index] = base
		}
		return &tupleValue{elements: bases}, true
	case "__mro__":
		order := class.methodResolutionOrder()
		values := make([]Value, len(order))
		for index, entry := range order {
			values[index] = entry
		}
		return &tupleValue{elements: values}, true
	default:
		return nil, false
	}
}

func setTypeAttribute(class *typeValue, name string, value Value) {
	class.namespace.values[name] = value
	text, isString := value.(*stringValue)
	if !isString {
		return
	}
	switch name {
	case "__name__":
		class.name = text.value
	case "__qualname__":
		class.qualifiedName = text.value
	case "__module__":
		class.module = text.value
	}
}

var _ Value = (*genericAliasValue)(nil)
var _ Value = (*unionValue)(nil)
