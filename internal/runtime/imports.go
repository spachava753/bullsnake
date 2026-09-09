package runtime

import (
	"slices"
	"strings"
)

// executeImportName validates compiler-supplied operands, then resumes or
// starts the ordered loading of one absolute module path.
func executeImportName(
	frame *frame,
	index int,
	name string,
) (instructionOutcome, error) {
	if request := frame.pendingImport; request != nil {
		frame.pendingImport = nil
		if request.instructionName != name {
			return instructionOutcome{}, frame.failure(index, "pending import name changed")
		}
		return advanceImport(frame, index, request)
	}
	fromList, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	level, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	levelValue, ok := level.(*intValue)
	if !ok {
		return instructionOutcome{}, frame.failure(index, "import level is not an integer")
	}
	absoluteName := name
	if levelValue.value.Sign() < 0 {
		return instructionOutcome{
			kind:      raised,
			exception: newException("ValueError", "level must be >= 0"),
		}, nil
	}
	if levelValue.value.Sign() > 0 {
		resolved, exception := resolveRelativeImport(frame, name, levelValue)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		absoluteName = resolved
	}
	if err := validateFromList(frame, index, fromList); err != nil {
		return instructionOutcome{}, err
	}
	if frame.runtime == nil {
		return instructionOutcome{}, frame.failure(index, "frame has no owning runtime")
	}
	return advanceImport(frame, index, newImportRequest(name, absoluteName, fromList))
}

// resolveRelativeImport applies the requested parent count to the executing
// module's package name and returns the absolute name used by ordinary loading.
func resolveRelativeImport(frame *frame, name string, level *intValue) (string, *Exception) {
	packageValue, found := frame.globals.get("__package__")
	packageName, ok := packageValue.(*stringValue)
	if found && !ok {
		return "", newException("TypeError", "__package__ not set to a string")
	}
	if !found || packageName.value == "" {
		return "", newException(
			"ImportError",
			"attempted relative import with no known parent package",
		)
	}
	parts := strings.Split(packageName.value, ".")
	if !level.value.IsInt64() || level.value.Int64() > int64(len(parts)) {
		return "", newException(
			"ImportError",
			"attempted relative import beyond top-level package",
		)
	}
	base := strings.Join(parts[:len(parts)-int(level.value.Int64())+1], ".")
	if name == "" {
		return base, nil
	}
	return base + "." + name, nil
}

func newImportRequest(instructionName, name string, fromList Value) *importRequest {
	names := qualifiedImportNames(name)
	returnName := names[0]
	var fromNames []string
	if values, ok := fromList.(*tupleValue); ok {
		returnName = name
		fromNames = make([]string, len(values.elements))
		for index, value := range values.elements {
			fromNames[index] = value.(*stringValue).value
		}
	}
	return &importRequest{
		instructionName: instructionName,
		requestedName:   name,
		names:           names,
		returnName:      returnName,
		fromNames:       fromNames,
	}
}

func qualifiedImportNames(name string) []string {
	parts := strings.Split(name, ".")
	names := make([]string, len(parts))
	for index := range parts {
		names[index] = strings.Join(parts[:index+1], ".")
	}
	return names
}

// advanceImport walks cached or loader-provided path components until it must
// suspend for a module frame or can push the import statement's selected module.
func advanceImport(
	frame *frame,
	index int,
	request *importRequest,
) (instructionOutcome, error) {
	for {
		for request.next < len(request.names) {
			name := request.names[request.next]
			var parent *Module
			if request.next != 0 {
				parent = frame.runtime.modules[request.names[request.next-1]]
				if parent == nil {
					return instructionOutcome{}, frame.failure(index, "import parent is not cached")
				}
				if !parent.isPackage {
					return instructionOutcome{
						kind: raised,
						exception: newException(
							"ModuleNotFoundError",
							"No module named '"+name+"'; '"+parent.name+"' is not a package",
						),
					}, nil
				}
			}
			if module, found := frame.runtime.modules[name]; found {
				if parent != nil {
					child := name[strings.LastIndexByte(name, '.')+1:]
					parent.globals.values[child] = module
				}
				request.next++
				continue
			}
			if constructor, found := frame.runtime.constructors[name]; found {
				_, exception, err := frame.runtime.initializeModule(name, constructor)
				if err != nil {
					return instructionOutcome{}, err
				}
				if exception != nil {
					return raiseOutcome(exception), nil
				}
				continue
			}
			if frame.runtime.loader == nil {
				if name == request.fallbackName {
					request.next++
					continue
				}
				return missingModuleOutcome(name), nil
			}
			loadRequest := ModuleRequest{Name: name}
			if parent != nil {
				loadRequest.SearchLocations = slices.Clone(parent.searchLocations)
			}
			spec, found, err := frame.runtime.loader(loadRequest)
			if err != nil {
				return instructionOutcome{}, err
			}
			if !found {
				if name == request.fallbackName {
					request.next++
					continue
				}
				return missingModuleOutcome(name), nil
			}
			module, imported, err := frame.runtime.newModuleFrame(name, spec, frame)
			if err != nil {
				return instructionOutcome{}, err
			}
			frame.runtime.modules[name] = module
			imported.moduleImport = &moduleImport{module: module, request: request}
			return instructionOutcome{kind: called, frame: imported}, nil
		}

		module := frame.runtime.modules[request.requestedName]
		if module == nil {
			return instructionOutcome{}, frame.failure(index, "completed import is not cached")
		}
		request.fallbackName = ""
		started, exception := startNextFromImport(module, request)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if started {
			continue
		}
		result := frame.runtime.modules[request.returnName]
		if result == nil {
			return instructionOutcome{}, frame.failure(index, "import result is not cached")
		}
		return pushOutcome(frame, index, result)
	}
}

// startNextFromImport expands package wildcard exports and selects the next
// missing from-list name that should be attempted as a child module.
func startNextFromImport(module *Module, request *importRequest) (bool, *Exception) {
	if !module.isPackage {
		return false, nil
	}
	for request.fromIndex < len(request.fromNames) {
		name := request.fromNames[request.fromIndex]
		request.fromIndex++
		if name == "*" {
			names, found, exception := moduleAllNames(module)
			if exception != nil {
				return false, exception
			}
			if found {
				for _, exported := range names {
					if exported != "*" {
						request.fromNames = append(request.fromNames, exported)
					}
				}
			}
			continue
		}
		if _, found := module.globals.get(name); found {
			continue
		}
		request.fallbackName = request.requestedName + "." + name
		request.names = qualifiedImportNames(request.fallbackName)
		request.next = strings.Count(request.requestedName, ".") + 1
		return true, nil
	}
	return false, nil
}

func missingModuleOutcome(name string) instructionOutcome {
	return instructionOutcome{
		kind:      raised,
		exception: newException("ModuleNotFoundError", "No module named '"+name+"'"),
	}
}

// moduleAllNames reads and validates the list or tuple used by the current
// wildcard-import subset while preserving whether __all__ was absent.
func moduleAllNames(module *Module) ([]string, bool, *Exception) {
	value, found := module.globals.get("__all__")
	if !found {
		return nil, false, nil
	}
	var elements []Value
	switch value := value.(type) {
	case *listValue:
		elements = value.elements
	case *tupleValue:
		elements = value.elements
	default:
		return nil, true, newException(
			"TypeError",
			module.name+".__all__ must be a list or tuple, not "+value.TypeName(),
		)
	}
	names := make([]string, len(elements))
	for index, element := range elements {
		name, ok := element.(*stringValue)
		if !ok {
			return nil, true, newException(
				"TypeError",
				"Item in "+module.name+".__all__ must be str, not "+element.TypeName(),
			)
		}
		names[index] = name.value
	}
	return names, true, nil
}

func validateFromList(frame *frame, index int, value Value) error {
	if value == None {
		return nil
	}
	names, ok := value.(*tupleValue)
	if !ok {
		return frame.failure(index, "import from-list is not a tuple")
	}
	for _, name := range names.elements {
		if _, ok := name.(*stringValue); !ok {
			return frame.failure(index, "import from-list item is not a string")
		}
	}
	return nil
}

func executeImportFrom(
	frame *frame,
	index int,
	name string,
) (instructionOutcome, error) {
	if len(frame.stack) == 0 {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	module, ok := frame.stack[len(frame.stack)-1].(*Module)
	if !ok {
		return instructionOutcome{}, frame.failure(index, "IMPORT_FROM owner is not a module")
	}
	value, found := module.globals.get(name)
	if !found {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"ImportError",
				"cannot import name '"+name+"' from '"+module.name+"' (unknown location)",
			),
		}, nil
	}
	return pushOutcome(frame, index, value)
}

// executeImportStar copies explicit __all__ names or public namespace names
// into the importing frame and reports invalid or missing explicit exports.
func executeImportStar(frame *frame, index int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	module, ok := value.(*Module)
	if !ok {
		return instructionOutcome{}, frame.failure(index, "IMPORT_STAR owner is not a module")
	}
	names, explicit, exception := moduleAllNames(module)
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	if !explicit {
		names = make([]string, 0, len(module.globals.values))
		for name := range module.globals.values {
			names = append(names, name)
		}
	}
	for _, name := range names {
		if !explicit && strings.HasPrefix(name, "_") {
			continue
		}
		imported, found := module.globals.get(name)
		if !found {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"AttributeError",
					"module '"+module.name+"' has no attribute '"+name+"'",
				),
			}, nil
		}
		frame.locals.values[name] = imported
	}
	return instructionOutcome{kind: advance}, nil
}

var _ Value = (*Module)(nil)
