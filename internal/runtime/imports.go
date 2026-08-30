package runtime

import "strings"

// executeImportName validates compiler-supplied level and from-list values,
// then returns or starts one flat absolute module in the owning runtime.
func executeImportName(
	frame *frame,
	index int,
	name string,
) (instructionOutcome, error) {
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
	if levelValue.value.Sign() != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"ImportError",
				"attempted relative import with no known parent package",
			),
		}, nil
	}
	if err := validateFromList(frame, index, fromList); err != nil {
		return instructionOutcome{}, err
	}
	if frame.runtime == nil {
		return instructionOutcome{}, frame.failure(index, "frame has no owning runtime")
	}
	if strings.Contains(name, ".") {
		return missingModuleOutcome(name), nil
	}
	if module, found := frame.runtime.modules[name]; found {
		return pushOutcome(frame, index, module)
	}
	if frame.runtime.loader == nil {
		return missingModuleOutcome(name), nil
	}
	code, found, err := frame.runtime.loader(name)
	if err != nil {
		return instructionOutcome{}, err
	}
	if !found {
		return missingModuleOutcome(name), nil
	}
	module, imported, err := frame.runtime.newModuleFrame(name, code, frame)
	if err != nil {
		return instructionOutcome{}, err
	}
	frame.runtime.modules[name] = module
	imported.importedModule = module
	return instructionOutcome{kind: called, frame: imported}, nil
}

func missingModuleOutcome(name string) instructionOutcome {
	return instructionOutcome{
		kind:      raised,
		exception: newException("ModuleNotFoundError", "No module named '"+name+"'"),
	}
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

func executeImportStar(frame *frame, index int) (instructionOutcome, error) {
	value, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(index, "operand stack underflow")
	}
	module, ok := value.(*Module)
	if !ok {
		return instructionOutcome{}, frame.failure(index, "IMPORT_STAR owner is not a module")
	}
	for name, imported := range module.globals.values {
		if !strings.HasPrefix(name, "_") {
			frame.locals.values[name] = imported
		}
	}
	return instructionOutcome{kind: advance}, nil
}

var _ Value = (*Module)(nil)
