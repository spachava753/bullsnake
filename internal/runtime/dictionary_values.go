package runtime

import (
	"strconv"
	"strings"
)

type dictionaryValuesMethod struct {
	dictionary *dictValue
}

type dictionaryValuesView struct {
	dictionary *dictValue
}

type dictionaryValuesIterator struct {
	dictionary *dictValue
	index      int
	length     int
	version    uint64
	failure    *Exception
}

func (*dictionaryValuesMethod) TypeName() string { return "builtin_function_or_method" }
func (*dictionaryValuesMethod) Repr() string {
	return "<built-in method values of dict object>"
}
func (*dictionaryValuesMethod) isValue() {}

func (*dictionaryValuesView) TypeName() string { return "dict_values" }
func (view *dictionaryValuesView) Repr() string {
	var builder strings.Builder
	builder.WriteString("dict_values([")
	for index, entry := range view.dictionary.entries {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(entry.value.Repr())
	}
	builder.WriteString("])")
	return builder.String()
}
func (*dictionaryValuesView) isValue() {}

func (*dictionaryValuesIterator) TypeName() string { return "dict_valueiterator" }
func (*dictionaryValuesIterator) Repr() string {
	return "<dict_valueiterator object>"
}
func (*dictionaryValuesIterator) isValue() {}

// next rejects key-set changes and reads each current value when its insertion
// position is reached, so value replacement remains visible during iteration.
func (iterator *dictionaryValuesIterator) next() (Value, bool, *Exception) {
	if iterator.failure != nil {
		return nil, false, iterator.failure
	}
	dictionary := iterator.dictionary
	if len(dictionary.entries) != iterator.length {
		iterator.failure = newException(
			"RuntimeError",
			"dictionary changed size during iteration",
		)
		return nil, false, iterator.failure
	}
	if dictionary.version != iterator.version {
		iterator.failure = newException(
			"RuntimeError",
			"dictionary keys changed during iteration",
		)
		return nil, false, iterator.failure
	}
	if iterator.index >= len(dictionary.entries) {
		return nil, false, nil
	}
	value := dictionary.entries[iterator.index].value
	iterator.index++
	return value, true, nil
}

func executeDictionaryValuesCall(
	caller *frame,
	instruction int,
	base int,
	method *dictionaryValuesMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.values() takes no keyword arguments",
		)), nil
	}
	if len(arguments) != 0 {
		count := len(arguments)
		discardCallSegment(caller, base)
		return raiseOutcome(newException(
			"TypeError",
			"dict.values() takes no arguments ("+strconv.Itoa(count)+" given)",
		)), nil
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &dictionaryValuesView{
		dictionary: method.dictionary,
	})
}
