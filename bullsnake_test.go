package bullsnake_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake"
	"github.com/spachava753/bullsnake/host"
)

type recordingFiles struct {
	host.FileSystem
	reads []string
}

func (files *recordingFiles) ReadFile(name string) ([]byte, error) {
	files.reads = append(files.reads, name)
	return files.FileSystem.ReadFile(name)
}

func TestInterpreterUsesConfiguredHost(t *testing.T) {
	root := t.TempDir()
	moduleFile := filepath.Join(root, "library.py")
	if err := os.WriteFile(moduleFile, []byte("value = 41\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := &recordingFiles{FileSystem: host.Default().Files}
	var output bytes.Buffer
	interpreter := bullsnake.New(bullsnake.Config{
		Host: host.Services{
			Files:  files,
			Stdout: &output,
		},
		ModuleSearchPath: []string{root},
	})
	module, err := interpreter.ExecuteModule(
		"main",
		"main.py",
		"import library\nimport sys\nanswer = library.value + 1\n"+
			"sys.stdout.write('ready')\n",
	)
	if err != nil {
		t.Fatal(err)
	}
	answer, found := module.Get("answer")
	if !found || answer.Repr() != "42" {
		t.Fatalf("answer = %v, %t, want 42", answer, found)
	}
	if output.String() != "ready" {
		t.Fatalf("stdout = %q, want ready", output.String())
	}
	if len(files.reads) == 0 {
		t.Fatal("source import bypassed the configured filesystem")
	}
}

func TestExecuteFileWithoutCapability(t *testing.T) {
	interpreter := bullsnake.New(bullsnake.Config{})
	module, err := interpreter.ExecuteFile("main", "main.py")
	if module != nil || !errors.Is(err, host.ErrDenied) {
		t.Fatalf("ExecuteFile = %#v, %v, want host.ErrDenied", module, err)
	}
}

func TestPinnedCPythonUnittestFiles(t *testing.T) {
	tests := []struct {
		path    string
		testRun string
	}{
		{path: "testdata/cpython/test_future_single_import.py", testRun: "Ran 3 tests"},
		{path: "testdata/cpython/test_future_multiple_imports.py", testRun: "Ran 1 test"},
		{path: "testdata/cpython/test_int_literal.py", testRun: "Ran 6 tests"},
	}
	for _, test := range tests {
		t.Run(filepath.Base(test.path), func(t *testing.T) {
			services := host.Default()
			var output bytes.Buffer
			services.Stderr = &output
			interpreter := bullsnake.New(bullsnake.Config{Host: services})
			module, err := interpreter.ExecuteFile("__main__", test.path)
			if err != nil {
				t.Fatal(err)
			}
			if module == nil {
				t.Fatal("CPython test execution returned no module")
			}
			if !strings.Contains(output.String(), test.testRun) ||
				!strings.Contains(output.String(), "OK") {
				t.Fatalf("unittest output = %q", output.String())
			}
		})
	}
}
