package runtime

func (function *functionValue) pythonClosure() Value {
	if len(function.closure) == 0 {
		return None
	}
	if function.closureTuple == nil {
		cells := make([]Value, len(function.closure))
		for index, cell := range function.closure {
			cells[index] = cell
		}
		function.closureTuple = &tupleValue{elements: cells}
	}
	return function.closureTuple
}

func readOnlyFunctionMetadata(name string) bool {
	switch name {
	case "__code__", "__closure__", "__globals__", "__type_params__", "__annotate__", "__annotations__":
		return true
	}
	return false
}

func executeCellAttributeLoad(caller *frame, instruction int, cell *cellValue, name string) (instructionOutcome, error) {
	if name != "cell_contents" {
		return raiseOutcome(newException("AttributeError", "'cell' object has no attribute '"+name+"'")), nil
	}
	if cell.value == nil {
		return raiseOutcome(newException("ValueError", "Cell is empty")), nil
	}
	return pushOutcome(caller, instruction, cell.value)
}

func executeCellAttributeStore(cell *cellValue, name string, value Value) (instructionOutcome, error) {
	if name != "cell_contents" {
		return raiseOutcome(newException("AttributeError", "'cell' object has no attribute '"+name+"'")), nil
	}
	cell.value = value
	return instructionOutcome{kind: advance}, nil
}
