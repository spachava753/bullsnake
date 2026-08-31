package runtime

import "strings"

type dictionaryKeysView struct {
	dictionary *dictValue
}

func (*dictionaryKeysView) TypeName() string { return "dict_keys" }
func (view *dictionaryKeysView) Repr() string {
	var builder strings.Builder
	builder.WriteString("dict_keys([")
	for index, entry := range view.dictionary.entries {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(entry.key.Repr())
	}
	builder.WriteString("])")
	return builder.String()
}
func (*dictionaryKeysView) isValue() {}

type dictionaryItemsView struct {
	dictionary *dictValue
}

func (*dictionaryItemsView) TypeName() string { return "dict_items" }
func (view *dictionaryItemsView) Repr() string {
	var builder strings.Builder
	builder.WriteString("dict_items([")
	for index, entry := range view.dictionary.entries {
		if index != 0 {
			builder.WriteString(", ")
		}
		builder.WriteByte('(')
		builder.WriteString(entry.key.Repr())
		builder.WriteString(", ")
		builder.WriteString(entry.value.Repr())
		builder.WriteByte(')')
	}
	builder.WriteString("])")
	return builder.String()
}
func (*dictionaryItemsView) isValue() {}

type dictionaryItemsIterator struct {
	dictionary *dictValue
	index      int
	length     int
	version    uint64
	failure    *Exception
}

func (*dictionaryItemsIterator) TypeName() string { return "dict_itemiterator" }
func (*dictionaryItemsIterator) Repr() string {
	return "<dict_itemiterator object>"
}
func (*dictionaryItemsIterator) isValue() {}

// next rejects key-set changes but reads each current value when its key is
// reached, so replacing a value during iteration remains visible and valid.
func (iterator *dictionaryItemsIterator) next() (Value, bool, *Exception) {
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
	entry := dictionary.entries[iterator.index]
	iterator.index++
	return &tupleValue{elements: []Value{entry.key, entry.value}}, true, nil
}
