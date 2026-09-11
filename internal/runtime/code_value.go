package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

// codeValue wraps one runtime-prepared code object without exposing mutable
// instructions or pretending that Bullsnake bytecode is CPython bytecode.
type codeValue struct {
	code       *preparedCode
	attributes *Namespace
}

func (*codeValue) TypeName() string { return "code" }
func (value *codeValue) Repr() string {
	return "<code object " + value.code.code.Name() + ">"
}
func (*codeValue) isValue() {}

// pythonCode publishes one stable wrapper per runtime-prepared code object.
// Function closures and retained frames therefore share the same code identity.
func (prepared *preparedCode) pythonCode() *codeValue {
	if prepared.pythonValue == nil {
		code := prepared.code
		locals := prepared.pythonLocalNames()
		metadata := newNamespace()
		for name, value := range map[string]string{
			"co_name": code.Name(), "co_qualname": code.QualifiedName(), "co_filename": code.Filename(),
		} {
			metadata.values[name] = &stringValue{value: value}
		}
		for name, value := range map[string]int{
			"co_firstlineno": code.FirstLine(), "co_argcount": code.PositionalCount(),
			"co_posonlyargcount": code.PositionalOnlyCount(), "co_kwonlyargcount": code.KeywordOnlyCount(),
			"co_nlocals": len(locals), "co_flags": pythonCodeFlags(code.Flags()),
		} {
			metadata.values[name] = integerFromInt64(int64(value))
		}
		for name, values := range map[string][]string{
			"co_names": prepared.names, "co_varnames": locals,
			"co_cellvars": prepared.cells, "co_freevars": prepared.freeVars,
		} {
			elements := make([]Value, len(values))
			for index, value := range values {
				elements[index] = &stringValue{value: value}
			}
			metadata.values[name] = &tupleValue{elements: elements}
		}
		prepared.pythonValue = &codeValue{code: prepared, attributes: metadata}
	}
	return prepared.pythonValue
}

// pythonLocalNames projects the VM's parameter slots into Python's positional,
// keyword-only, varargs, kwargs order and excludes non-parameter cell-only names.
func (prepared *preparedCode) pythonLocalNames() []string {
	positional := prepared.code.PositionalCount()
	keywordStart, keywordEnd := keywordOnlyRange(prepared)
	result := append([]string(nil), prepared.locals[:positional]...)
	result = append(result, prepared.locals[keywordStart:keywordEnd]...)
	if prepared.code.Flags()&bytecode.VarArgs != 0 {
		result = append(result, prepared.locals[positional])
	}
	parameterEnd := keywordEnd
	if prepared.code.Flags()&bytecode.VarKeywords != 0 {
		result = append(result, prepared.locals[parameterEnd])
		parameterEnd++
	}
	for _, name := range prepared.locals[parameterEnd:] {
		cell := false
		for _, cellName := range prepared.cells {
			cell = cell || cellName == name
		}
		if !cell {
			result = append(result, name)
		}
	}
	return result
}

// pythonCodeFlags maps supported execution properties to Python's flag bits;
// the compiler's internal coroutine/async-generator bit positions differ.
func pythonCodeFlags(flags bytecode.CodeFlags) int {
	result := 0
	for flag, bit := range map[bytecode.CodeFlags]int{
		bytecode.Optimized: 0x01, bytecode.NewLocals: 0x02,
		bytecode.VarArgs: 0x04, bytecode.VarKeywords: 0x08,
		bytecode.Nested: 0x10, bytecode.Generator: 0x20,
		bytecode.Coroutine: 0x80, bytecode.AsyncGenerator: 0x200,
	} {
		if flags&flag != 0 {
			result |= bit
		}
	}
	return result
}

func executeCodeAttributeLoad(caller *frame, instruction int, value *codeValue, name string) (instructionOutcome, error) {
	if attribute, found := value.attributes.get(name); found {
		return pushOutcome(caller, instruction, attribute)
	}
	return raiseOutcome(newException("AttributeError", "'code' object has no attribute '"+name+"'")), nil
}
