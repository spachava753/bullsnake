package runtime

type templateValue struct {
	text string
}

func (*templateValue) TypeName() string { return "string.templatelib.Template" }
func (template *templateValue) Repr() string {
	return "Template(strings=(" + (&stringValue{value: template.text}).Repr() + ",), interpolations=())"
}
func (*templateValue) isValue() {}
func (template *templateValue) attribute(name string) (Value, bool) {
	switch name {
	case "strings":
		return &tupleValue{elements: []Value{&stringValue{value: template.text}}}, true
	case "interpolations":
		return &tupleValue{}, true
	case "values":
		return &tupleValue{}, true
	default:
		return nil, false
	}
}

var templateType = &builtinTypeValue{
	name:    "string.templatelib.Template",
	matches: func(value Value) bool { _, ok := value.(*templateValue); return ok },
}

func builtinTemplate(_ *frame, arguments []Value) (Value, *Exception, error) {
	text, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "template payload must be a string"), nil
	}
	return &templateValue{text: text.value}, nil, nil
}

var _ Value = (*templateValue)(nil)
