package runtime

type dictViewKind uint8

const (
	dictKeysView dictViewKind = iota
	dictValuesView
	dictItemsView
)

type dictViewValue struct {
	dictionary *dictValue
	kind       dictViewKind
}

func (view *dictViewValue) TypeName() string { return dictViewTypes[view.kind].name }
func (view *dictViewValue) Repr() string     { return "<" + view.TypeName() + ">" }
func (*dictViewValue) isValue()              {}

var dictViewTypes = [3]*builtinTypeValue{
	{name: "dict_keys", matches: func(value Value) bool { view, ok := value.(*dictViewValue); return ok && view.kind == dictKeysView }},
	{name: "dict_values", matches: func(value Value) bool { view, ok := value.(*dictViewValue); return ok && view.kind == dictValuesView }},
	{name: "dict_items", matches: func(value Value) bool { view, ok := value.(*dictViewValue); return ok && view.kind == dictItemsView }},
}

type dictViewIterator struct {
	view  *dictViewValue
	index int
}

func (iterator *dictViewIterator) TypeName() string {
	switch iterator.view.kind {
	case dictValuesView:
		return "dict_valueiterator"
	case dictItemsView:
		return "dict_itemiterator"
	default:
		return "dict_keyiterator"
	}
}
func (iterator *dictViewIterator) Repr() string { return "<" + iterator.TypeName() + " object>" }
func (*dictViewIterator) isValue()              {}
func (iterator *dictViewIterator) next() (Value, bool, *Exception) {
	if iterator.index >= len(iterator.view.dictionary.entries) {
		return nil, false, nil
	}
	entry := iterator.view.dictionary.entries[iterator.index]
	iterator.index++
	switch iterator.view.kind {
	case dictValuesView:
		return entry.value, true, nil
	case dictItemsView:
		return &tupleValue{elements: []Value{entry.key, entry.value}}, true, nil
	default:
		return entry.key, true, nil
	}
}

var _ Value = (*dictViewValue)(nil)
var _ valueIterator = (*dictViewIterator)(nil)
