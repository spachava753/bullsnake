package runtime

import "strings"

// executeImportName validates compiler-supplied level and from-list values,
// then resolves one completed flat module from the frame's owning runtime.
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
	module, found := frame.runtime.modules[name]
	if !found || strings.Contains(name, ".") {
		return instructionOutcome{
			kind:      raised,
			exception: newException("ModuleNotFoundError", "No module named '"+name+"'"),
		}, nil
	}
	return pushOutcome(frame, index, module)
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
