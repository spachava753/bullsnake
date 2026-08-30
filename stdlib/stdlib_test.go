package stdlib_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/importer"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

func TestVendoredStandardLibrary(t *testing.T) {
	paths, err := filepath.Glob("3.14/tests/test_*.py")
	if err != nil {
		t.Fatal(err)
	}
	loader := importer.NewFileSystem("3.14/tests", "3.14")
	for _, path := range paths {
		moduleName := strings.TrimSuffix(filepath.Base(path), ".py")
		t.Run(moduleName, func(t *testing.T) {
			spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: moduleName})
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatalf("vendored %s module was not found", moduleName)
			}
			if _, err := bullruntime.NewWithLoader(loader.Load).ExecuteModuleSpec(
				moduleName,
				spec,
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}
