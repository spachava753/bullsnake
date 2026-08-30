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
	writeSource(t, first, "main.py", "import library\nimport package.child\nfrom package import sibling\ntry:\n    from package import absent\nexcept ImportError:\n    missing_member = True\ntry:\n    from package import broken\nexcept ModuleNotFoundError:\n    nested_missing = True\nanswer = library.value + 1\npackage_answer = package.child.value\nsibling_answer = sibling.value\n")
	writeSource(t, second, "library.py", "value = 41\n")
	writeSource(t, second, "package/__init__.py", "marker = 'package'\n")
	writeSource(t, second, "package/child.py", "value = 42\n")
	writeSource(t, second, "package/sibling.py", "value = 43\n")
	writeSource(t, second, "package/broken.py", "import hidden_dependency\n")
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
		module, err := runtime.ExecuteModule("main", spec.Code)
		if err != nil {
			t.Fatal(err)
		}
		answer, found := module.Get("answer")
		if !found || answer.Repr() != "42" {
			t.Fatalf("answer = %v, %t, want 42", answer, found)
		}
		packageAnswer, found := module.Get("package_answer")
		if !found || packageAnswer.Repr() != "42" {
			t.Fatalf("package_answer = %v, %t, want 42", packageAnswer, found)
		}
		assertImportValue(t, module, "sibling_answer", "43")
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
