package runtime

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spachava753/bullsnake/host"
)

func (runtimeState *Runtime) newSubprocessModule() *Module {
	module := newSystemModule("subprocess", "")
	module.globals.values["PIPE"] = newInt64(-1)
	module.globals.values["STDOUT"] = newInt64(-2)
	module.globals.values["DEVNULL"] = newInt64(-3)
	module.globals.values["Popen"] = nativeKeywordAwareFunctionNamed(
		"subprocess.Popen", 1, 1, runtimeState.newPythonProcess,
	)
	return module
}

// newPythonProcess validates a Popen request and captures its script and working directory.
func (runtimeState *Runtime) newPythonProcess(
	caller *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	values, exception := iterableElements(arguments[0], "Popen")
	if exception != nil {
		return nil, exception, nil
	}
	argv := make([]string, len(values))
	for index, value := range values {
		text, ok := value.(*stringValue)
		if !ok {
			return nil, newException("TypeError", "Popen arguments must be strings"), nil
		}
		argv[index] = text.value
	}
	cwd := ""
	if runtimeState.host.Process != nil {
		cwd, _ = runtimeState.host.Process.Getwd()
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name, ok := entry.key.(*stringValue)
			if !ok {
				return nil, newException("TypeError", "Popen keywords must be strings"), nil
			}
			if name.value == "cwd" {
				selected, ok := entry.value.(*stringValue)
				if !ok {
					return nil, newException("TypeError", "cwd must be str"), nil
				}
				if filepath.IsAbs(selected.value) || cwd == "" {
					cwd = selected.value
				} else {
					cwd = filepath.Join(cwd, selected.value)
				}
			}
		}
	}
	return &pythonProcessValue{runtime: runtimeState, argv: argv, cwd: cwd}, nil, nil
}

type pythonProcessValue struct {
	runtime    *Runtime
	argv       []string
	cwd        string
	stdout     string
	stderr     string
	returnCode int64
	executed   bool
}

func (*pythonProcessValue) TypeName() string { return "Popen" }
func (*pythonProcessValue) Repr() string     { return "<Popen object>" }
func (*pythonProcessValue) isValue()         {}

// attribute exposes the Popen lifecycle and captured return-code surface.
func (process *pythonProcessValue) attribute(name string) (Value, bool) {
	switch name {
	case "communicate":
		return nativeFunctionNamed("Popen.communicate", 0, 2, process.communicate), true
	case "wait":
		return nativeFunctionNamed("Popen.wait", 0, 1, process.wait), true
	case "poll":
		return nativeFunctionNamed("Popen.poll", 0, 0, process.poll), true
	case "__enter__":
		return nativeFunctionNamed("Popen.__enter__", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) { return process, nil, nil }), true
	case "__exit__":
		return nativeFunctionNamed("Popen.__exit__", 3, 3,
			func(_ *frame, _ []Value) (Value, *Exception, error) { return falseSingleton, nil, nil }), true
	case "returncode":
		if !process.executed {
			return None, true
		}
		return newInt64(process.returnCode), true
	default:
		return nil, false
	}
}

func (process *pythonProcessValue) communicate(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	process.run()
	return &tupleValue{elements: []Value{
		&bytesValue{value: process.stdout},
		&bytesValue{value: process.stderr},
	}}, nil, nil
}

func (process *pythonProcessValue) wait(_ *frame, _ []Value) (Value, *Exception, error) {
	process.run()
	return newInt64(process.returnCode), nil, nil
}

func (process *pythonProcessValue) poll(_ *frame, _ []Value) (Value, *Exception, error) {
	if !process.executed {
		return None, nil, nil
	}
	return newInt64(process.returnCode), nil, nil
}

// run executes one requested Python script in an isolated child runtime exactly once.
func (process *pythonProcessValue) run() {
	if process.executed {
		return
	}
	process.executed = true
	if process.runtime.host.Files == nil || process.runtime.compiler == nil {
		process.returnCode = 1
		process.stderr = host.ErrDenied.Error()
		return
	}
	scriptIndex, moduleIndex := -1, -1
	for index, argument := range process.argv {
		if strings.HasSuffix(argument, ".py") {
			scriptIndex = index
			break
		}
		if argument == "-m" && index+1 < len(process.argv) {
			moduleIndex = index
			break
		}
	}
	if scriptIndex < 0 && moduleIndex < 0 {
		process.returnCode = 1
		process.stderr = "Popen requires a Python script or module"
		return
	}
	filename := "<-m>"
	source := ""
	argumentStart := 0
	if moduleIndex >= 0 {
		moduleName := process.argv[moduleIndex+1]
		if moduleName != "unittest" {
			process.returnCode = 1
			process.stderr = "Popen module is not supported: " + moduleName
			return
		}
		filename = "<-m unittest>"
		source = "import unittest\nunittest.main(module=None)\n"
		argumentStart = moduleIndex + 2
	} else {
		filename = process.argv[scriptIndex]
		if !filepath.IsAbs(filename) {
			filename = filepath.Join(process.cwd, filename)
		}
		data, err := process.runtime.host.Files.ReadFile(filename)
		if err != nil {
			process.returnCode = 1
			process.stderr = err.Error()
			return
		}
		source = string(data)
		argumentStart = scriptIndex + 1
	}
	code, err := process.runtime.compiler(filename, source)
	if err != nil {
		process.returnCode = 1
		process.stderr = err.Error()
		return
	}
	var stdout, stderr bytes.Buffer
	childHost := process.runtime.host
	childHost.Stdout = &stdout
	childHost.Stderr = &stderr
	childArguments := append([]string{filename}, process.argv[argumentStart:]...)
	childHost.Process = childProcess{
		parent: childHost.Process,
		args:   childArguments,
		cwd:    process.cwd,
	}
	parentLoader := process.runtime.loader
	parentWorkingDirectory := ""
	if process.runtime.host.Process != nil {
		parentWorkingDirectory, _ = process.runtime.host.Process.Getwd()
	}
	overlayLoader := func(request ModuleRequest) (ModuleSpec, bool, error) {
		if request.SearchLocations == nil && process.cwd != "" && !strings.Contains(request.Name, ".") {
			packageFile := filepath.Join(process.cwd, request.Name, "__init__.py")
			data, readErr := process.runtime.host.Files.ReadFile(packageFile)
			if readErr == nil {
				loadedCode, compileErr := process.runtime.compiler(packageFile, string(data))
				return ModuleSpec{
					Code: loadedCode, IsPackage: true, Origin: packageFile,
					SearchLocations: []string{filepath.Dir(packageFile)},
				}, true, compileErr
			}
			if !errors.Is(readErr, fs.ErrNotExist) {
				return ModuleSpec{}, true, readErr
			}
			moduleFile := filepath.Join(process.cwd, request.Name+".py")
			data, readErr = process.runtime.host.Files.ReadFile(moduleFile)
			if readErr == nil {
				loadedCode, compileErr := process.runtime.compiler(moduleFile, string(data))
				return ModuleSpec{Code: loadedCode, Origin: moduleFile}, true, compileErr
			}
			if !errors.Is(readErr, fs.ErrNotExist) {
				return ModuleSpec{}, true, readErr
			}
		}
		if parentLoader == nil {
			return ModuleSpec{}, false, nil
		}
		spec, found, loadErr := parentLoader(request)
		if found && loadErr == nil && parentWorkingDirectory != "" {
			if spec.Origin != "" && !filepath.IsAbs(spec.Origin) {
				spec.Origin = filepath.Join(parentWorkingDirectory, spec.Origin)
			}
			for index, location := range spec.SearchLocations {
				if !filepath.IsAbs(location) {
					spec.SearchLocations[index] = filepath.Join(parentWorkingDirectory, location)
				}
			}
		}
		return spec, found, loadErr
	}
	child := newRuntime(Config{
		Loader: overlayLoader, Path: process.runtime.path,
		Host: childHost, Compiler: process.runtime.compiler,
	})
	if process.cwd != "" {
		path := child.modules["sys"].globals.values["path"].(*listValue)
		path.elements = append([]Value{&stringValue{value: process.cwd}}, path.elements...)
	}
	warnOptions := make([]Value, 0)
	optionEnd := scriptIndex
	if moduleIndex >= 0 {
		optionEnd = moduleIndex
	}
	for index := 1; index < optionEnd; index++ {
		argument := process.argv[index]
		if strings.HasPrefix(argument, "-W") && len(argument) > 2 {
			warnOptions = append(warnOptions, &stringValue{value: argument[2:]})
		}
	}
	child.modules["sys"].globals.values["warnoptions"] = &listValue{elements: warnOptions}
	_, err = child.ExecuteModuleSpec("__main__", ModuleSpec{Code: code, Origin: filename})
	process.stdout, process.stderr = stdout.String(), stderr.String()
	if err != nil {
		if uncaught, ok := err.(*UncaughtException); ok && uncaught.exception.class == systemExitType {
			code := uncaught.exception.code
			if code == nil || code == None || code == falseSingleton || valuesEqual(code, newInt64(0)) {
				process.returnCode = 0
			} else {
				process.returnCode = 1
			}
			return
		}
		process.returnCode = 1
		if process.stderr != "" && !strings.HasSuffix(process.stderr, "\n") {
			process.stderr += "\n"
		}
		process.stderr += err.Error()
	}
}

type childProcess struct {
	parent host.Process
	args   []string
	cwd    string
}

func (process childProcess) Args() []string { return append([]string(nil), process.args...) }
func (process childProcess) Environ() []string {
	if process.parent == nil {
		return nil
	}
	return process.parent.Environ()
}
func (process childProcess) Executable() (string, error) {
	if process.parent == nil {
		return "", errors.New("executable is unavailable")
	}
	return process.parent.Executable()
}
func (process childProcess) Getwd() (string, error) { return process.cwd, nil }

var _ Value = (*pythonProcessValue)(nil)
var _ host.Process = childProcess{}
