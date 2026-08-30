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
	writeSource(t, first, "main.py", "import library\nanswer = library.value + 1\n")
	writeSource(t, second, "library.py", "value = 41\n")
	writeSource(t, first, "broken.py", "if True\n")
	loader := importer.NewFileSystem(first, second)

	t.Run("executes source modules", func(t *testing.T) {
		spec, found, err := loader.Load("main")
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
		if _, found := runtime.Module("library"); !found {
			t.Fatal("imported library was not cached")
		}
	})

	t.Run("reports a missing module", func(t *testing.T) {
		spec, found, err := loader.Load("absent")
		if err != nil || found || spec.Code != nil {
			t.Fatalf("Load(absent) = %#v, %t, %v, want empty, false, nil", spec, found, err)
		}
	})

	t.Run("rejects non-flat names", func(t *testing.T) {
		for _, name := range []string{"", "package.child", "../main", `..\main`} {
			spec, found, err := loader.Load(name)
			if err != nil || found || spec.Code != nil {
				t.Fatalf("Load(%q) = %#v, %t, %v, want empty, false, nil", name, spec, found, err)
			}
		}
	})

	t.Run("preserves frontend errors", func(t *testing.T) {
		spec, found, err := loader.Load("broken")
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

func writeSource(t *testing.T, root, name, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}
