package stdlib_test

import (
	"errors"
	"testing"

	"github.com/spachava753/bullsnake/internal/importer"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

// TestNativeImportRecovery checks shared cache rollback and continuation cleanup
// after either a source exception or a host loader failure in a native import.
func TestNativeImportRecovery(t *testing.T) {
	for _, hostFailure := range []bool{false, true} {
		name := "source"
		if hostFailure {
			name = "host"
		}
		t.Run(name, func(t *testing.T) {
			loader := importer.NewFileSystem("testdata", "3.14")
			failure, _, err := loader.Load(bullruntime.ModuleRequest{Name: "object_state_failure"})
			if err != nil {
				t.Fatal(err)
			}
			loads := 0
			hostError := errors.New("source provider failed")
			runtime := bullruntime.NewWithLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
				if request.Name == "copyreg" {
					loads++
					if loads == 1 {
						if hostFailure {
							return bullruntime.ModuleSpec{}, false, hostError
						}
						return failure, true, nil
					}
				}
				return loader.Load(request)
			})
			entry := "object_state_retry"
			if hostFailure {
				entry = "object_state_import"
			}
			spec, _, err := loader.Load(bullruntime.ModuleRequest{Name: entry})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runtime.ExecuteModuleSpec(entry, spec)
			if hostFailure {
				if !errors.Is(err, hostError) {
					t.Fatalf("first execution error = %v, want host failure", err)
				}
				if _, found := runtime.Module("copyreg"); found {
					t.Fatal("failed import remained cached")
				}
				_, err = runtime.ExecuteModuleSpec(entry, spec)
			}
			if err != nil {
				t.Fatal(err)
			}
			if loads != 2 {
				t.Fatalf("copyreg loads = %d, want failed attempt then successful retry", loads)
			}
			if !hostFailure {
				if _, found := runtime.Module("operator"); !found {
					t.Fatal("completed nested import was removed")
				}
			}
			if _, found := runtime.Module("copyreg"); !found {
				t.Fatal("successful native import was not cached")
			}
		})
	}
}
