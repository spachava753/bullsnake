package runtime_test

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/spachava753/bullsnake/host"
	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

type fakeClock struct {
	now       time.Time
	monotonic time.Duration
	slept     []time.Duration
	err       error
}

type fixedTimeZone struct {
	location *time.Location
	calls    int
}

func (zone *fixedTimeZone) LocalTime(instant time.Time) (time.Time, error) {
	zone.calls++
	return instant.In(zone.location), nil
}

type fileGuard struct {
	host.FileSystem
	reads int
}

type recordingFileMutator struct {
	removed     []string
	removedDirs []string
	err         error
}

type recordingWorkingDirectory struct {
	names []string
	err   error
}

func (directory *recordingWorkingDirectory) Chdir(name string) error {
	directory.names = append(directory.names, name)
	return directory.err
}

func (mutator *recordingFileMutator) Remove(name string) error {
	mutator.removed = append(mutator.removed, name)
	return mutator.err
}

func (mutator *recordingFileMutator) RemoveDir(name string) error {
	mutator.removedDirs = append(mutator.removedDirs, name)
	return mutator.err
}

func (guard *fileGuard) ReadDir(string) ([]fs.DirEntry, error) {
	guard.reads++
	return nil, host.ErrDenied
}

func (clock *fakeClock) Now() (time.Time, error) {
	return clock.now, clock.err
}

func (clock *fakeClock) Monotonic() (time.Duration, error) {
	return clock.monotonic, clock.err
}

func (clock *fakeClock) Sleep(duration time.Duration) error {
	clock.slept = append(clock.slept, duration)
	return clock.err
}

func TestConfiguredSystemModules(t *testing.T) {
	clock := &fakeClock{
		now:       time.Unix(123, 500_000_000),
		monotonic: 2500 * time.Millisecond,
	}
	zone := &fixedTimeZone{location: time.FixedZone("test", 60*60)}
	directory := &recordingWorkingDirectory{}
	var output bytes.Buffer
	runtime := bullruntime.NewWithConfig(bullruntime.Config{
		Path: []string{"/stdlib", "/application"},
		Host: host.Services{
			Clock:            clock,
			TimeZone:         zone,
			WorkingDirectory: directory,
			Stdout:           &output,
		},
	})
	module, err := runtime.ExecuteModule("configured", compileSource(t,
		"import os\n"+
			"import sys\n"+
			"import time\n"+
			"wall = time.time()\n"+
			"counter = time.perf_counter()\n"+
			"local = time.localtime(0)\n"+
			"stamp = time.ctime(0)\n"+
			"time.sleep(1.5)\n"+
			"os.chdir('/sandbox')\n"+
			"written = sys.stdout.write('ready')\n"+
			"paths = sys.path\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, module, "wall", "123.5")
	assertModuleRepr(t, module, "counter", "2.5")
	assertModuleRepr(t, module, "local", "(1970, 1, 1, 1, 0, 0, 3, 1, 0)")
	assertModuleRepr(t, module, "stamp", "'Thu Jan  1 01:00:00 1970'")
	assertModuleRepr(t, module, "written", "5")
	assertModuleRepr(t, module, "paths", "['/stdlib', '/application']")
	if output.String() != "ready" {
		t.Fatalf("stdout = %q, want ready", output.String())
	}
	if len(clock.slept) != 1 || clock.slept[0] != 1500*time.Millisecond {
		t.Fatalf("sleep calls = %v, want [1.5s]", clock.slept)
	}
	if zone.calls != 2 {
		t.Fatalf("time zone calls = %d, want 2", zone.calls)
	}
	if len(directory.names) != 1 || directory.names[0] != "/sandbox" {
		t.Fatalf("working-directory calls = %v, want [/sandbox]", directory.names)
	}
}

func TestStandardLibraryRuntimePrimitives(t *testing.T) {
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("primitives", compileSource(t, `
values = [3, 1, 2]
values.sort(key=lambda value: -value)

class Display:
    def __repr__(self):
        return "custom"

class Duration:
    def __init__(self, value):
        self.value = value
    def __sub__(self, other):
        return Duration(self.value - other.value)
    def __abs__(self):
        return Duration(abs(self.value))
    def __le__(self, other):
        return self.value <= other.value

display = repr(Display())
difference = abs(Duration(2) - Duration(5)).value
ordered = Duration(2) <= Duration(3)
items = [1, 2, 3]
items[:] = [4, 5]
subset = {1} <= {1, 2}
division = divmod(-5, 2)
number = 1.5 + 2j
`))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"values":     "[3, 2, 1]",
		"display":    "'custom'",
		"difference": "3",
		"ordered":    "True",
		"items":      "[4, 5]",
		"subset":     "True",
		"division":   "(-3, 1)",
		"number":     "(1.5+2j)",
	} {
		assertModuleRepr(t, module, name, want)
	}
}

func TestMissingHostCapabilityIsDenied(t *testing.T) {
	runtime := bullruntime.NewWithConfig(bullruntime.Config{})
	module, err := runtime.ExecuteModule("denied", compileSource(t,
		"import time\ntime.time()\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "PermissionError" {
		t.Fatalf("exception type = %q, want PermissionError", got)
	}
}

func TestSystemModuleHostPolicyIsEnforced(t *testing.T) {
	files := &fileGuard{FileSystem: host.Default().Files}
	runtime := bullruntime.NewWithConfig(bullruntime.Config{
		Host: host.Services{Files: files},
	})
	module, err := runtime.ExecuteModule("denied", compileSource(t,
		"import os\nos.listdir('.')\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "PermissionError" {
		t.Fatalf("exception type = %q, want PermissionError", got)
	}
	if files.reads != 1 {
		t.Fatalf("directory reads = %d, want 1", files.reads)
	}
}

func TestRandomBytesUseHostEntropy(t *testing.T) {
	runtime := bullruntime.NewWithConfig(bullruntime.Config{
		Host: host.Services{Entropy: bytes.NewReader([]byte{0x00, 0x7f, 0x80, 0xff})},
	})
	module, err := runtime.ExecuteModule("entropy", compileSource(t,
		"import os\nrandom_bytes = os.urandom(4)\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, module, "random_bytes", `b'\x00\x7f\x80\xff'`)
}

func TestEntropyDenialIsPermissionError(t *testing.T) {
	runtime := bullruntime.NewWithConfig(bullruntime.Config{})
	module, err := runtime.ExecuteModule("denied_entropy", compileSource(t,
		"import os\nos.urandom(1)\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "PermissionError" {
		t.Fatalf("exception type = %q, want PermissionError", got)
	}
}

func TestRemovalCapabilityRoutesCalls(t *testing.T) {
	mutator := &recordingFileMutator{}
	runtime := bullruntime.NewWithConfig(bullruntime.Config{
		Host: host.Services{FileMutator: mutator},
	})
	_, err := runtime.ExecuteModule("mutations", compileSource(t,
		"import os\nos.unlink('/virtual/file')\nos.rmdir('/virtual/dir')\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := mutator.removed; len(got) != 1 || got[0] != "/virtual/file" {
		t.Fatalf("removed files = %v, want [/virtual/file]", got)
	}
	if got := mutator.removedDirs; len(got) != 1 || got[0] != "/virtual/dir" {
		t.Fatalf("removed directories = %v, want [/virtual/dir]", got)
	}
}

func TestDeniedRemovalRaisesPermissionError(t *testing.T) {
	runtime := bullruntime.NewWithConfig(bullruntime.Config{})
	module, err := runtime.ExecuteModule("denied", compileSource(t,
		"import os\nos.unlink('/virtual/file')\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "PermissionError" {
		t.Fatalf("exception type = %q, want PermissionError", got)
	}
}

func TestModuleCacheAPI(t *testing.T) {
	code := compileSource(t, "answer = 42\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("cached", code)
	if err != nil {
		t.Fatal(err)
	}
	if module.Name() != "cached" {
		t.Fatalf("module name = %q, want cached", module.Name())
	}
	cached, ok := runtime.Module("cached")
	if !ok || cached != module {
		t.Fatal("successful module was not cached by identity")
	}
}

func TestFunctionValueMetadata(t *testing.T) {
	code := compileSource(t, "def add(left, right):\n    return left + right\n")
	module, err := bullruntime.New().ExecuteModule("functions", code)
	if err != nil {
		t.Fatal(err)
	}
	function, ok := module.Get("add")
	if !ok {
		t.Fatal("module has no add binding")
	}
	if got := function.TypeName(); got != "function" {
		t.Errorf("add type = %q, want function", got)
	}
}

func TestCachedModuleImports(t *testing.T) {
	libraryCode := compileSource(t, "value = 41\n"+
		"public = 'ready'\n"+
		"_private = 'hidden'\n")
	runtime := bullruntime.New()
	if _, err := runtime.ExecuteModule("library", libraryCode); err != nil {
		t.Fatal(err)
	}
	mainCode := compileSource(t, "import library as first\n"+
		"import library as second\n"+
		"from library import public as selected\n"+
		"from library import *\n"+
		"def read():\n"+
		"    import library\n"+
		"    return library.value\n"+
		"answer = first.value + 1\n"+
		"same = first is second\n"+
		"function_value = read()\n"+
		"module_repr = f'{first!r}'\n")
	module, err := runtime.ExecuteModule("main", mainCode)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"answer":         "42",
		"same":           "True",
		"selected":       "'ready'",
		"public":         "'ready'",
		"function_value": "41",
		"module_repr":    `"<module 'library'>"`,
	}
	for name, expected := range want {
		value, ok := module.Get(name)
		if !ok {
			t.Fatalf("module has no %q binding", name)
		}
		if got := value.Repr(); got != expected {
			t.Errorf("%s = %s, want %s", name, got, expected)
		}
	}
	if _, leaked := module.Get("_private"); leaked {
		t.Fatal("star import copied a private binding")
	}
}

func TestImportedModuleMutation(t *testing.T) {
	runtime := bullruntime.New()
	libraryCode := compileSource(t, "value = 1\nremovable = 2\n")
	library, err := runtime.ExecuteModule("library", libraryCode)
	if err != nil {
		t.Fatal(err)
	}
	mutatorCode := compileSource(t, "import library\n"+
		"library.value = 3\n"+
		"del library.removable\n")
	if _, err := runtime.ExecuteModule("mutator", mutatorCode); err != nil {
		t.Fatal(err)
	}
	value, ok := library.Get("value")
	if !ok || value.Repr() != "3" {
		t.Fatalf("library value = %v, %t, want 3", value, ok)
	}
	if _, found := library.Get("removable"); found {
		t.Fatal("deleted module member remained cached")
	}
	observerCode := compileSource(t, "import library\nobserved = library.value\n")
	observer, err := runtime.ExecuteModule("observer", observerCode)
	if err != nil {
		t.Fatal(err)
	}
	observed, ok := observer.Get("observed")
	if !ok || observed.Repr() != "3" {
		t.Fatalf("observed = %v, %t, want 3", observed, ok)
	}
}

func TestPreloadedImportFailures(t *testing.T) {
	runtime := bullruntime.New()
	libraryCode := compileSource(t, "present = 1\n")
	if _, err := runtime.ExecuteModule("library", libraryCode); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		source      string
		wantType    string
		wantMessage string
	}{
		{
			name:        "member",
			source:      "from library import missing\n",
			wantType:    "ImportError",
			wantMessage: "cannot import name 'missing' from 'library' (unknown location)",
		},
		{
			name:        "deleted member",
			source:      "import library\ndel library.missing\n",
			wantType:    "AttributeError",
			wantMessage: "module 'library' has no attribute 'missing'",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := compileSource(t, test.source)
			module, err := runtime.ExecuteModule("failure_"+test.name, code)
			if module != nil {
				t.Fatalf("module = %#v, want nil", module)
			}
			var raised *bullruntime.UncaughtException
			if !errors.As(err, &raised) {
				t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
			}
			if got := raised.Exception().TypeName(); got != test.wantType {
				t.Errorf("exception type = %q, want %q", got, test.wantType)
			}
			if got := raised.Exception().Message(); got != test.wantMessage {
				t.Errorf("exception message = %q, want %q", got, test.wantMessage)
			}
		})
	}
}

func TestLoadedModuleIdentity(t *testing.T) {
	loads := make(map[string]int)
	sources := map[string]string{
		"helper":  "value = 40\n",
		"library": "import helper\nvalue = helper.value + 2\n",
	}
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		source, found := sources[name]
		if !found {
			return bullruntime.ModuleSpec{}, false, nil
		}
		loads[name]++
		return bullruntime.ModuleSpec{Code: compileSource(t, source)}, true, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t,
		"import library as first\n"+
			"import library as second\n"+
			"answer = first.value\n"+
			"same = first is second\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, module, "answer", "42")
	assertModuleRepr(t, module, "same", "True")
	if loads["library"] != 1 || loads["helper"] != 1 {
		t.Fatalf("load counts = %v, want library:1 helper:1", loads)
	}
	library, found := runtime.Module("library")
	if !found {
		t.Fatal("loaded library is absent from the runtime cache")
	}
	first, _ := module.Get("first")
	if first != library {
		t.Fatal("import did not retain the cached module identity")
	}
}

func TestCircularModuleInitialization(t *testing.T) {
	loads := make(map[string]int)
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		loads[name]++
		if name != "beta" {
			return bullruntime.ModuleSpec{}, false, nil
		}
		return bullruntime.ModuleSpec{
			Code: compileSource(t, "import alpha\nobserved = alpha.state\n"),
		}, true, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	alpha, err := runtime.ExecuteModule("alpha", compileSource(t,
		"state = 'starting'\n"+
			"import beta\n"+
			"observed = beta.observed\n"+
			"state = 'finished'\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, alpha, "observed", "'starting'")
	assertModuleRepr(t, alpha, "state", "'finished'")
	if loads["alpha"] != 0 || loads["beta"] != 1 {
		t.Fatalf("load counts = %v, want alpha:0 beta:1", loads)
	}
}

func TestFailedModuleInitialization(t *testing.T) {
	loads := make(map[string]int)
	sources := map[string]string{
		"side":   "value = 1\n",
		"broken": "import side\nraise ValueError('boom')\n",
	}
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		source, found := sources[name]
		if !found {
			return bullruntime.ModuleSpec{}, false, nil
		}
		loads[name]++
		return bullruntime.ModuleSpec{Code: compileSource(t, source)}, true, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t,
		"import sys\n"+
			"attempts = 0\n"+
			"try:\n"+
			"    import broken\n"+
			"except ValueError:\n"+
			"    attempts = attempts + 1\n"+
			"try:\n"+
			"    import broken\n"+
			"except ValueError:\n"+
			"    attempts = attempts + 1\n"+
			"broken_cached = 'broken' in sys.modules\n"))
	if err != nil {
		t.Fatal(err)
	}
	assertModuleRepr(t, module, "attempts", "2")
	if loads["broken"] != 2 || loads["side"] != 1 {
		t.Fatalf("load counts = %v, want broken:2 side:1", loads)
	}
	assertModuleRepr(t, module, "broken_cached", "False")
	if _, found := runtime.Module("broken"); found {
		t.Fatal("failed module remained in the runtime cache")
	}
	if _, found := runtime.Module("side"); !found {
		t.Fatal("successful side import was removed after its importer failed")
	}
}

func TestLoaderHostFailure(t *testing.T) {
	loadFailure := errors.New("module storage unavailable")
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		switch name {
		case "bridge":
			return bullruntime.ModuleSpec{
				Code: compileSource(t, "import unavailable\n"),
			}, true, nil
		case "unavailable":
			return bullruntime.ModuleSpec{}, false, loadFailure
		default:
			return bullruntime.ModuleSpec{}, false, nil
		}
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t, "import bridge\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	if !errors.Is(err, loadFailure) {
		t.Fatalf("error = %v, want loader failure", err)
	}
	for _, name := range []string{"main", "bridge", "unavailable"} {
		if _, found := runtime.Module(name); found {
			t.Fatalf("failed module %q remained in the runtime cache", name)
		}
	}
}

func TestDottedImports(t *testing.T) {
	loads := make(map[string]int)
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		loads[name]++
		switch name {
		case "package":
			if request.SearchLocations != nil {
				t.Fatalf("top-level search locations = %v, want nil", request.SearchLocations)
			}
			return bullruntime.ModuleSpec{
				Code:            compileSource(t, "marker = 'root'\n"),
				IsPackage:       true,
				Origin:          "/modules/package/__init__.py",
				SearchLocations: []string{"/modules/package"},
			}, true, nil
		case "package.child":
			if len(request.SearchLocations) != 1 || request.SearchLocations[0] != "/modules/package" {
				t.Fatalf("child search locations = %v, want package path", request.SearchLocations)
			}
			return bullruntime.ModuleSpec{
				Code:   compileSource(t, "value = 42\n"),
				Origin: "/modules/package/child.py",
			}, true, nil
		default:
			return bullruntime.ModuleSpec{}, false, nil
		}
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t,
		"import package.child\n"+
			"import package.child as alias\n"+
			"from package.child import value as selected\n"+
			"answer = package.child.value\n"+
			"same = alias is package.child\n"+
			"root_package = package.__package__\n"+
			"child_package = alias.__package__\n"+
			"root_path = package.__path__\n"+
			"child_file = alias.__file__\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"answer":        "42",
		"same":          "True",
		"selected":      "42",
		"root_package":  "'package'",
		"child_package": "'package'",
		"root_path":     "['/modules/package']",
		"child_file":    "'/modules/package/child.py'",
	}
	for name, expected := range want {
		assertModuleRepr(t, module, name, expected)
	}
	if loads["package"] != 1 || loads["package.child"] != 1 {
		t.Fatalf("load counts = %v, want each package component once", loads)
	}
	parent, _ := runtime.Module("package")
	child, _ := runtime.Module("package.child")
	published, found := parent.Get("child")
	if !found || published != child {
		t.Fatal("child module was not published on its parent by identity")
	}
}

func TestNonPackageImportParent(t *testing.T) {
	loads := make(map[string]int)
	loader := bullruntime.ModuleLoader(func(request bullruntime.ModuleRequest) (bullruntime.ModuleSpec, bool, error) {
		name := request.Name
		loads[name]++
		if name == "plain" {
			return bullruntime.ModuleSpec{Code: compileSource(t, "value = 1\n")}, true, nil
		}
		return bullruntime.ModuleSpec{}, false, nil
	})
	runtime := bullruntime.NewWithLoader(loader)
	module, err := runtime.ExecuteModule("main", compileSource(t, "import plain.child\n"))
	if module != nil {
		t.Fatalf("module = %#v, want nil", module)
	}
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "ModuleNotFoundError" {
		t.Fatalf("exception type = %q, want ModuleNotFoundError", got)
	}
	want := "No module named 'plain.child'; 'plain' is not a package"
	if got := raised.Exception().Message(); got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if loads["plain"] != 1 || loads["plain.child"] != 0 {
		t.Fatalf("load counts = %v, want plain:1 plain.child:0", loads)
	}
	if _, found := runtime.Module("plain"); !found {
		t.Fatal("successfully loaded parent was removed after child lookup failed")
	}
}

func assertModuleRepr(t *testing.T, module *bullruntime.Module, name, want string) {
	t.Helper()
	value, found := module.Get(name)
	if !found {
		t.Fatalf("module has no %q binding", name)
	}
	if got := value.Repr(); got != want {
		t.Fatalf("%s = %s, want %s", name, got, want)
	}
}

func TestReraiseKeepsOriginalLocation(t *testing.T) {
	code := testCodeSpec(bytecode.CodeSpec{
		StackSize: 1,
		Instructions: []bytecode.Instruction{
			{Opcode: bytecode.LoadAssertionError},
			{Opcode: bytecode.RaiseVarargs, Operand: 1},
			{Opcode: bytecode.Reraise},
		},
		ExceptionHandlers: []bytecode.ExceptionHandler{
			{Start: 0, End: 2, Target: 2},
		},
	})
	_, err := bullruntime.New().ExecuteModule("reraised", code)
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Error(); got != "<broken>:1:2: AssertionError: " {
		t.Fatalf("error = %q, want original raise location", got)
	}
}

func TestExplicitRaiseMovesOrigin(t *testing.T) {
	code := compileSource(t, "error = ValueError('stored')\n"+
		"try:\n"+
		"    raise error\n"+
		"except:\n"+
		"    pass\n"+
		"raise error\n")
	_, err := bullruntime.New().ExecuteModule("explicit", code)
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Error(); got != "<test>:6:1: ValueError: stored" {
		t.Fatalf("error = %q, want second explicit raise location", got)
	}
}

func TestUncaughtTracebackAPI(t *testing.T) {
	type wantFrame struct {
		name string
		line int
	}
	tests := []struct {
		name          string
		source        string
		wantFrames    []wantFrame
		wantBacktrace string
	}{
		{
			name: "nested calls",
			source: "def inner():\n" +
				"    missing_name\n" +
				"def middle():\n" +
				"    inner()\n" +
				"def outer():\n" +
				"    middle()\n" +
				"outer()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 7},
				{name: "outer", line: 6},
				{name: "middle", line: 4},
				{name: "inner", line: 2},
			},
			wantBacktrace: "Traceback (most recent call last):\n" +
				"  File \"<test>\", line 7, in <module>\n" +
				"  File \"<test>\", line 6, in outer\n" +
				"  File \"<test>\", line 4, in middle\n" +
				"  File \"<test>\", line 2, in inner\n" +
				"NameError: name 'missing_name' is not defined",
		},
		{
			name: "bare reraise",
			source: "def inner():\n" +
				"    try:\n" +
				"        missing_name\n" +
				"    except NameError:\n" +
				"        raise\n" +
				"inner()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 6},
				{name: "inner", line: 3},
			},
		},
		{
			name: "explicit reraise",
			source: "def inner():\n" +
				"    try:\n" +
				"        missing_name\n" +
				"    except NameError as error:\n" +
				"        raise error\n" +
				"inner()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 6},
				{name: "inner", line: 5},
				{name: "inner", line: 3},
			},
		},
		{
			name: "called bare reraise",
			source: "def reraiser():\n" +
				"    raise\n" +
				"def outer():\n" +
				"    try:\n" +
				"        missing_name\n" +
				"    except NameError:\n" +
				"        reraiser()\n" +
				"outer()\n",
			wantFrames: []wantFrame{
				{name: "<module>", line: 8},
				{name: "outer", line: 7},
				{name: "outer", line: 5},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := bullruntime.New().ExecuteModule("traceback", compileSource(t, test.source))
			var raised *bullruntime.UncaughtException
			if !errors.As(err, &raised) {
				t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
			}
			frames := raised.Traceback()
			if len(frames) != len(test.wantFrames) {
				t.Fatalf("traceback has %d frames, want %d: %#v", len(frames), len(test.wantFrames), frames)
			}
			for index, want := range test.wantFrames {
				frame := frames[index]
				if frame.Filename != "<test>" || frame.Name != want.name || frame.Span.Start.Line != want.line {
					t.Errorf("frame %d = %#v, want <test>:%d in %s", index, frame, want.line, want.name)
				}
			}
			if test.wantBacktrace != "" && raised.Backtrace() != test.wantBacktrace {
				t.Errorf("backtrace = %q, want %q", raised.Backtrace(), test.wantBacktrace)
			}
			frames[0].Name = "changed"
			if raised.Traceback()[0].Name == "changed" {
				t.Fatal("Traceback returned mutable internal storage")
			}
		})
	}
}

func TestScalarConstants(t *testing.T) {
	code := compileSource(t, "none_value = None\n"+
		"false_value = False\n"+
		"true_value = True\n"+
		"ellipsis_value = ...\n"+
		"float_value = 1.25\n"+
		"imaginary_value = 2j\n"+
		"text_value = 'line\\nsnowman: \\N{SNOWMAN}'\n"+
		"surrogate_value = '\\ud800'\n"+
		"bytes_value = b'\\x00A\\xff'\n")
	runtime := bullruntime.New()
	module, err := runtime.ExecuteModule("scalars", code)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]struct {
		typeName string
		repr     string
	}{
		"none_value":      {typeName: "NoneType", repr: "None"},
		"false_value":     {typeName: "bool", repr: "False"},
		"true_value":      {typeName: "bool", repr: "True"},
		"ellipsis_value":  {typeName: "ellipsis", repr: "Ellipsis"},
		"float_value":     {typeName: "float", repr: "1.25"},
		"imaginary_value": {typeName: "complex", repr: "2j"},
		"text_value":      {typeName: "str", repr: "'line\\nsnowman: \u2603'"},
		"surrogate_value": {typeName: "str", repr: "'\\ud800'"},
		"bytes_value":     {typeName: "bytes", repr: "b'\\x00A\\xff'"},
	}

	for name, expected := range want {
		value, ok := module.Get(name)
		if !ok {
			t.Fatalf("module has no %q binding", name)
		}
		if got := value.TypeName(); got != expected.typeName {
			t.Errorf("%s type = %q, want %q", name, got, expected.typeName)
		}
		if got := value.Repr(); got != expected.repr {
			t.Errorf("%s repr = %q, want %q", name, got, expected.repr)
		}
	}

	first, _ := module.Get("true_value")
	secondCode := compileSource(t, "other = True\n")
	secondModule, err := runtime.ExecuteModule("other", secondCode)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := secondModule.Get("other")
	if first != second {
		t.Fatal("True did not retain singleton identity across code objects")
	}
}

func compileSource(t *testing.T, source string) *bytecode.Code {
	t.Helper()
	module, err := parser.Parse("<test>", source)
	if err != nil {
		t.Fatal(err)
	}
	table, err := resolver.Resolve("<test>", module)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile("<test>", module, table)
	if err != nil {
		t.Fatal(err)
	}
	return code
}
