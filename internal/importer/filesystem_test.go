package importer_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/importer"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

func TestFileSystem(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	writeSource(t, first, "main.py",
		"import library\n"+
			"import package.child\n"+
			"import package.sub.module\n"+
			"from package import sibling\n"+
			"from exports import *\n"+
			"try:\n"+
			"    hidden\n"+
			"except NameError:\n"+
			"    hidden_omitted = True\n"+
			"try:\n"+
			"    from package import absent\n"+
			"except ImportError:\n"+
			"    missing_member = True\n"+
			"try:\n"+
			"    from package import broken\n"+
			"except ModuleNotFoundError:\n"+
			"    nested_missing = True\n"+
			"answer = library.value + 1\n"+
			"package_answer = package.child.value\n"+
			"relative_answer = package.sub.module.answer\n"+
			"sibling_answer = sibling.value\n"+
			"export_total = public + _private + child.value\n")
	writeSource(t, second, "library.py", "value = 41\n")
	writeSource(t, second, "package/__init__.py", "marker = 'package'\n")
	writeSource(t, second, "package/child.py", "value = 42\n")
	writeSource(t, second, "package/sibling.py", "value = 43\n")
	writeSource(t, second, "package/broken.py", "import hidden_dependency\n")
	writeSource(t, second, "package/sub/__init__.py", "")
	writeSource(t, second, "package/sub/peer.py", "value = 1\n")
	writeSource(t, second, "package/sub/module.py",
		"from . import peer\n"+
			"from .. import sibling\n"+
			"from ..child import value as child_value\n"+
			"answer = peer.value + sibling.value + child_value\n")
	writeSource(t, second, "package/beyond.py", "from .. import nowhere\n")
	writeSource(t, second, "exports/__init__.py",
		"__all__ = ['public', '_private', 'child']\n"+
			"public = 1\n"+
			"_private = 2\n"+
			"hidden = 3\n")
	writeSource(t, second, "exports/child.py", "value = 4\n")
	writeSource(t, second, "entry_package/__init__.py",
		"from .child import value\n"+
			"answer = value + 1\n")
	writeSource(t, second, "entry_package/child.py", "value = 41\n")
	writeSource(t, second, "invalid_all.py", "__all__ = [1]\n")
	writeSource(t, first, "invalid_all_entry.py", "from invalid_all import *\n")
	writeSource(t, first, "beyond_entry.py", "import package.beyond\n")
	writeSource(t, first, "relative_entry.py", "from . import nowhere\n")
	writeSource(t, first, "package/child.py", "value = -1\n")
	writeSource(t, first, "choice.py", "kind = 'module'\n")
	writeSource(t, first, "choice/__init__.py", "kind = 'package'\n")
	writeSource(t, first, "broken.py", "if True\n")
	loader := importer.NewFileSystem(first, second)

	t.Run("executes source modules", func(t *testing.T) {
		spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: "main"})
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatal("main module was not found")
		}
		runtime := bullruntime.NewWithLoader(loader.Load)
		module, err := runtime.ExecuteModuleSpec("main", spec)
		if err != nil {
			t.Fatal(err)
		}
		assertImportValue(t, module, "__file__", "'"+filepath.Join(first, "main.py")+"'")
		assertImportValue(t, module, "__package__", "''")
		answer, found := module.Get("answer")
		if !found || answer.Repr() != "42" {
			t.Fatalf("answer = %v, %t, want 42", answer, found)
		}
		packageAnswer, found := module.Get("package_answer")
		if !found || packageAnswer.Repr() != "42" {
			t.Fatalf("package_answer = %v, %t, want 42", packageAnswer, found)
		}
		assertImportValue(t, module, "sibling_answer", "43")
		assertImportValue(t, module, "relative_answer", "86")
		assertImportValue(t, module, "export_total", "7")
		assertImportValue(t, module, "hidden_omitted", "True")
		assertImportValue(t, module, "missing_member", "True")
		assertImportValue(t, module, "nested_missing", "True")
		if _, found := runtime.Module("library"); !found {
			t.Fatal("imported library was not cached")
		}
		packageModule, found := runtime.Module("package")
		if !found {
			t.Fatal("imported package was not cached")
		}
		assertImportValue(t, packageModule, "__package__", "'package'")
		assertImportValue(t, packageModule, "__file__", "'"+filepath.Join(second, "package", "__init__.py")+"'")
		assertImportValue(t, packageModule, "__path__", "['"+filepath.Join(second, "package")+"']")
		child, found := runtime.Module("package.child")
		if !found {
			t.Fatal("imported package child was not cached")
		}
		assertImportValue(t, child, "__package__", "'package'")
		if _, found := runtime.Module("package.sibling"); !found {
			t.Fatal("from-list submodule was not cached")
		}
		if _, found := runtime.Module("package.broken"); found {
			t.Fatal("failing from-list submodule remained cached")
		}
		if _, found := runtime.Module("exports.child"); !found {
			t.Fatal("package __all__ did not load its child module")
		}
	})

	t.Run("executes a package entry", func(t *testing.T) {
		spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: "entry_package"})
		if err != nil || !found {
			t.Fatalf("Load(entry_package) = %#v, %t, %v", spec, found, err)
		}
		runtime := bullruntime.NewWithLoader(loader.Load)
		module, err := runtime.ExecuteModuleSpec("entry_package", spec)
		if err != nil {
			t.Fatal(err)
		}
		assertImportValue(t, module, "answer", "42")
		assertImportValue(t, module, "__package__", "'entry_package'")
		assertImportValue(t, module, "__path__", "['"+filepath.Join(second, "entry_package")+"']")
		if _, found := runtime.Module("entry_package.child"); !found {
			t.Fatal("package entry did not load its relative child")
		}
	})

	t.Run("reports relative import errors", func(t *testing.T) {
		tests := []struct {
			module  string
			message string
		}{
			{
				module:  "relative_entry",
				message: "attempted relative import with no known parent package",
			},
			{
				module:  "beyond_entry",
				message: "attempted relative import beyond top-level package",
			},
		}
		for _, test := range tests {
			t.Run(test.module, func(t *testing.T) {
				spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: test.module})
				if err != nil || !found {
					t.Fatalf("Load(%s) = %#v, %t, %v", test.module, spec, found, err)
				}
				_, err = bullruntime.NewWithLoader(loader.Load).ExecuteModule(test.module, spec.Code)
				var raised *bullruntime.UncaughtException
				if !errors.As(err, &raised) {
					t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
				}
				if got := raised.Exception().TypeName(); got != "ImportError" {
					t.Fatalf("exception type = %q, want ImportError", got)
				}
				if got := raised.Exception().Message(); got != test.message {
					t.Fatalf("message = %q, want %q", got, test.message)
				}
			})
		}
	})

	t.Run("rejects invalid all entries", func(t *testing.T) {
		spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: "invalid_all_entry"})
		if err != nil || !found {
			t.Fatalf("Load(invalid_all_entry) = %#v, %t, %v", spec, found, err)
		}
		_, err = bullruntime.NewWithLoader(loader.Load).ExecuteModule("invalid_all_entry", spec.Code)
		var raised *bullruntime.UncaughtException
		if !errors.As(err, &raised) {
			t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
		}
		if got := raised.Exception().TypeName(); got != "TypeError" {
			t.Fatalf("exception type = %q, want TypeError", got)
		}
		want := "Item in invalid_all.__all__ must be str, not int"
		if got := raised.Exception().Message(); got != want {
			t.Fatalf("message = %q, want %q", got, want)
		}
	})

	t.Run("prefers a package directory", func(t *testing.T) {
		spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: "choice"})
		if err != nil || !found {
			t.Fatalf("Load(choice) = %#v, %t, %v, want package", spec, found, err)
		}
		if !spec.IsPackage {
			t.Fatal("choice.py won over choice/__init__.py")
		}
		want := filepath.Join(first, "choice", "__init__.py")
		if spec.Origin != want {
			t.Fatalf("origin = %q, want %q", spec.Origin, want)
		}
	})

	t.Run("reports a missing module", func(t *testing.T) {
		spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: "absent"})
		if err != nil || found || spec.Code != nil {
			t.Fatalf("Load(absent) = %#v, %t, %v, want empty, false, nil", spec, found, err)
		}
	})

	t.Run("rejects invalid module names", func(t *testing.T) {
		for _, name := range []string{"", "package..child", "../main", `..\main`} {
			spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: name})
			if err != nil || found || spec.Code != nil {
				t.Fatalf("Load(%q) = %#v, %t, %v, want empty, false, nil", name, spec, found, err)
			}
		}
	})

	t.Run("preserves frontend errors", func(t *testing.T) {
		spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: "broken"})
		if spec.Code != nil || !found {
			t.Fatalf("Load(broken) = %#v, %t, %v, want empty, true, error", spec, found, err)
		}
		var parseError *parser.Error
		if !errors.As(err, &parseError) {
			t.Fatalf("error = %T %v, want wrapped *parser.Error", err, err)
		}
		if parseError.Filename != filepath.Join(first, "broken.py") {
			t.Fatalf("filename = %q, want broken module path", parseError.Filename)
		}
	})
}

func assertImportValue(t *testing.T, module *bullruntime.Module, name, want string) {
	t.Helper()
	value, found := module.Get(name)
	if !found || value.Repr() != want {
		t.Fatalf("%s = %v, %t, want %s", name, value, found, want)
	}
}

func writeSource(t *testing.T, root, name, source string) {
	t.Helper()
	filename := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}
