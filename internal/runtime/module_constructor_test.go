package runtime

import (
	"errors"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

func testConstructorRegistration(t *testing.T) {
	runtime := New()
	constructor := moduleConstructor{initialize: func(*Runtime, *Module) (*Exception, error) { return nil, nil }}
	if err := runtime.registerModule("native", constructor); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native", "builtins", "sys", ""} {
		if err := runtime.registerModule(name, constructor); err == nil {
			t.Fatalf("accepted duplicate or invalid %q", name)
		}
	}
	if err := runtime.registerModule("invalid", moduleConstructor{}); err == nil {
		t.Fatal("accepted nil constructor")
	}
}

func testConstructorCircularIdentityAndFailureCleanup(t *testing.T) {
	for _, hostFailure := range []bool{false, true} {
		runtime := New()
		attempts := 0
		sentinel := errors.New("provider initialization failed")
		dependency := moduleConstructor{initialize: func(*Runtime, *Module) (*Exception, error) { return nil, nil }}
		var constructor moduleConstructor
		constructor.initialize = func(runtime *Runtime, module *Module) (*Exception, error) {
			attempts++
			cached, exception, err := runtime.initializeModule("native", constructor)
			if cached != module || exception != nil || err != nil {
				t.Fatal("circular initialization lost identity")
			}
			if _, exception, err := runtime.initializeModule("dependency", dependency); exception != nil || err != nil {
				t.Fatal("dependency failed")
			}
			if attempts == 1 {
				if hostFailure {
					return nil, sentinel
				}
				return newException("ValueError", "initialization failed"), nil
			}
			return nil, nil
		}
		module, exception, err := runtime.initializeModule("native", constructor)
		if module != nil || (exception == nil && err == nil) {
			t.Fatal("initialization did not fail")
		}
		if hostFailure && !errors.Is(err, sentinel) {
			t.Fatal("host error lost")
		}
		if _, found := runtime.Module("native"); found {
			t.Fatal("failed module cached")
		}
		if _, found := runtime.Module("dependency"); !found {
			t.Fatal("completed dependency removed")
		}
		module, exception, err = runtime.initializeModule("native", constructor)
		if module == nil || exception != nil || err != nil || attempts != 2 {
			t.Fatal("retry failed")
		}
	}
}

func testConstructorPackageBinding(t *testing.T) {
	runtime := New()
	constructor := moduleConstructor{isPackage: true, initialize: func(*Runtime, *Module) (*Exception, error) { return nil, nil }}
	for _, name := range []string{"native", "native.child"} {
		if err := runtime.registerModule(name, constructor); err != nil {
			t.Fatal(err)
		}
	}
	source := "import native.child\nfrom native import child\nassert native.child is child\nassert child.__package__ == 'native.child'\n"
	module, err := parser.Parse("native.py", source)
	if err != nil {
		t.Fatal(err)
	}
	table, err := resolver.Resolve("native.py", module)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile("native.py", module, table)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteModule("check", code); err != nil {
		t.Fatal(err)
	}
}

func TestModuleConstructors(t *testing.T) {
	t.Run("registration", testConstructorRegistration)
	t.Run("circular identity and cleanup", testConstructorCircularIdentityAndFailureCleanup)
	t.Run("package binding", testConstructorPackageBinding)
}
