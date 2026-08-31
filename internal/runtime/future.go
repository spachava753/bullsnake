package runtime

const futureModuleName = "__future__"

var futureFeatureNames = []string{
	"nested_scopes",
	"generators",
	"division",
	"absolute_import",
	"with_statement",
	"print_function",
	"unicode_literals",
	"barry_as_FLUFL",
	"generator_stop",
	"annotations",
}

type futureFeatureValue struct {
	name string
}

func (*futureFeatureValue) TypeName() string { return "_Feature" }
func (feature *futureFeatureValue) Repr() string {
	return "<future feature " + feature.name + ">"
}
func (*futureFeatureValue) isValue() {}

func newFutureModule() *Module {
	globals := newNamespace()
	globals.values["__name__"] = &stringValue{value: futureModuleName}
	globals.values["__package__"] = &stringValue{value: ""}
	for _, name := range futureFeatureNames {
		globals.values[name] = &futureFeatureValue{name: name}
	}
	return &Module{name: futureModuleName, globals: globals}
}
