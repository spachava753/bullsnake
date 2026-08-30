package stdlib_test

import (
	"testing"

	"github.com/spachava753/bullsnake/internal/importer"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

func TestColorsys(t *testing.T) {
	loader := importer.NewFileSystem("3.14/tests", "3.14")
	spec, found, err := loader.Load(bullruntime.ModuleRequest{Name: "test_colorsys"})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("vendored test_colorsys module was not found")
	}
	if _, err := bullruntime.NewWithLoader(loader.Load).ExecuteModuleSpec(
		"test_colorsys",
		spec,
	); err != nil {
		t.Fatal(err)
	}
}
