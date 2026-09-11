package runtime

import (
	"os"
	goruntime "runtime"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

func runABCRegistryFixture(t *testing.T, runtime *Runtime, name string) *Module {
	t.Helper()
	path := "testdata/host/abc_" + name + ".py"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	syntax, err := parser.Parse(path, string(source))
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := resolver.Resolve(path, syntax)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(path, syntax, symbols)
	if err != nil {
		t.Fatal(err)
	}
	module, err := runtime.ExecuteModule(name, code)
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func testABCRegistryOwnership(t *testing.T) {
	runtime := New()
	module := runABCRegistryFixture(t, runtime, "registry")
	base := module.globals.values["Base"].(*typeValue)
	data := base.namespace.values["_abc_impl"].(*abcData)
	var references []weakClass
	for _, name := range []string{"Registered", "Positive", "Negative"} {
		references = append(references, makeWeakClass(module.globals.values[name]))
		module.globals.delete(name)
	}
	for range 10 {
		goruntime.GC()
		if references[0].value() == nil && references[1].value() == nil && references[2].value() == nil {
			break
		}
	}
	for index, reference := range references {
		if reference.value() != nil {
			t.Fatalf("ABC registry/cache retained class %d", index)
		}
	}
	for _, set := range []*weakClassSet{&data.registry, &data.positive, &data.negative, &base.subclasses} {
		if len(set.snapshot()) != 0 {
			t.Fatal("dead ABC entry was not pruned")
		}
	}
	runABCRegistryFixture(t, runtime, "collected")
	goruntime.KeepAlive(runtime)
	goruntime.KeepAlive(module)
}

func testABCRegistryIsolation(t *testing.T) {
	first, second := New(), New()
	firstModule := runABCRegistryFixture(t, first, "registry")
	if second.abcToken != 0 {
		t.Fatal("registration changed another runtime's token")
	}
	secondModule := runABCRegistryFixture(t, second, "registry")
	firstBase := firstModule.globals.values["Base"].(*typeValue)
	secondBase := secondModule.globals.values["Base"].(*typeValue)
	if firstBase.namespace.values["_abc_impl"] == secondBase.namespace.values["_abc_impl"] {
		t.Fatal("runtimes shared ABC data")
	}
	if first.abcToken != second.abcToken {
		t.Fatal("fresh runtimes did not independently advance their tokens")
	}
}

func TestABCRegistry(t *testing.T) {
	t.Run("ownership", testABCRegistryOwnership)
	t.Run("isolation", testABCRegistryIsolation)
}
