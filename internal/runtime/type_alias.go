package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

type noDefaultValue struct{}

func (*noDefaultValue) TypeName() string { return "NoDefaultType" }
func (*noDefaultValue) Repr() string     { return "typing.NoDefault" }
func (*noDefaultValue) isValue()         {}

var noDefaultSingleton = &noDefaultValue{}

type typeVarValue struct {
	name                 string
	boundEvaluator       *functionValue
	bound                Value
	boundEvaluated       bool
	constraintsEvaluator *functionValue
	constraints          Value
	constraintsEvaluated bool
	defaultEvaluator     *functionValue
	defaultValue         Value
	defaultEvaluated     bool
}

func (*typeVarValue) TypeName() string { return "typing.TypeVar" }
func (variable *typeVarValue) Repr() string {
	return variable.name
}
func (*typeVarValue) isValue() {}

type typeVarTupleValue struct {
	name string
}

func (*typeVarTupleValue) TypeName() string { return "typing.TypeVarTuple" }
func (variable *typeVarTupleValue) Repr() string {
	return variable.name
}
func (*typeVarTupleValue) isValue() {}

type paramSpecValue struct {
	name string
}

func (*paramSpecValue) TypeName() string { return "typing.ParamSpec" }
func (parameter *paramSpecValue) Repr() string {
	return parameter.name
}
func (*paramSpecValue) isValue() {}

type paramSpecArgsValue struct {
	parameter *paramSpecValue
}

func (*paramSpecArgsValue) TypeName() string { return "typing.ParamSpecArgs" }
func (arguments *paramSpecArgsValue) Repr() string {
	return arguments.parameter.name + ".args"
}
func (*paramSpecArgsValue) isValue() {}

type paramSpecKwargsValue struct {
	parameter *paramSpecValue
}

func (*paramSpecKwargsValue) TypeName() string { return "typing.ParamSpecKwargs" }
func (arguments *paramSpecKwargsValue) Repr() string {
	return arguments.parameter.name + ".kwargs"
}
func (*paramSpecKwargsValue) isValue() {}

type typeVarLoadKind uint8

const (
	typeVarBoundLoad typeVarLoadKind = iota
	typeVarConstraintsLoad
	typeVarDefaultLoad
)

type typeVarLoad struct {
	variable    *typeVarValue
	kind        typeVarLoadKind
	instruction int
}

type typeAliasValue struct {
	name       string
	module     Value
	typeParams *tupleValue
	compute    *functionValue
	value      Value
	evaluated  bool
}

func (*typeAliasValue) TypeName() string { return "typing.TypeAliasType" }
func (alias *typeAliasValue) Repr() string {
	return alias.name
}
func (*typeAliasValue) isValue() {}

type typeAliasLoad struct {
	alias       *typeAliasValue
	instruction int
}

// executeMakeTypeParameter creates one inferred-variance PEP 695 parameter.
func executeMakeTypeParameter(
	frame *frame,
	instruction int,
	opcode bytecode.Opcode,
) (instructionOutcome, error) {
	nameValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	name, ok := nameValue.(*stringValue)
	if !ok {
		kind := "type variable"
		if opcode == bytecode.MakeTypeVarTuple {
			kind = "type variable tuple"
		} else if opcode == bytecode.MakeParamSpec {
			kind = "parameter specification"
		}
		return instructionOutcome{}, frame.failure(
			instruction,
			kind+" name is not a string",
		)
	}
	var parameter Value = &typeVarValue{name: name.value}
	if opcode == bytecode.MakeTypeVarTuple {
		parameter = &typeVarTupleValue{name: name.value}
	} else if opcode == bytecode.MakeParamSpec {
		parameter = &paramSpecValue{name: name.value}
	}
	return pushOutcome(frame, instruction, parameter)
}

// executeSetTypeVarEvaluator attaches one compiler-created lazy evaluator and
// keeps the TypeVar on the stack for its hidden generic scope.
func executeSetTypeVarEvaluator(
	frame *frame,
	instruction int,
	kind typeVarLoadKind,
) (instructionOutcome, error) {
	evaluatorValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	variableValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	evaluator, ok := evaluatorValue.(*functionValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"type variable evaluator payload is not a function",
		)
	}
	variable, ok := variableValue.(*typeVarValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"type variable evaluator target is not a TypeVar",
		)
	}
	switch kind {
	case typeVarBoundLoad:
		variable.boundEvaluator = evaluator
	case typeVarConstraintsLoad:
		variable.constraintsEvaluator = evaluator
	case typeVarDefaultLoad:
		variable.defaultEvaluator = evaluator
	}
	return pushOutcome(frame, instruction, variable)
}

// executeTypeVarLoad returns a cached TypeVar attribute or starts its hidden
// evaluator through the ordinary frame loop.
func executeTypeVarLoad(
	frame *frame,
	instruction int,
	variable *typeVarValue,
	kind typeVarLoadKind,
) (instructionOutcome, error) {
	var evaluator *functionValue
	switch kind {
	case typeVarBoundLoad:
		if variable.boundEvaluated {
			return pushOutcome(frame, instruction, variable.bound)
		}
		evaluator = variable.boundEvaluator
	case typeVarConstraintsLoad:
		if variable.constraintsEvaluated {
			return pushOutcome(frame, instruction, variable.constraints)
		}
		evaluator = variable.constraintsEvaluator
	case typeVarDefaultLoad:
		if variable.defaultEvaluated {
			return pushOutcome(frame, instruction, variable.defaultValue)
		}
		evaluator = variable.defaultEvaluator
	}
	if evaluator == nil {
		switch kind {
		case typeVarConstraintsLoad:
			return pushOutcome(frame, instruction, &tupleValue{})
		case typeVarDefaultLoad:
			return pushOutcome(frame, instruction, noDefaultSingleton)
		default:
			return pushOutcome(frame, instruction, None)
		}
	}
	if evaluator.code.code.Flags()&bytecode.Generator != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"type variable evaluator returned a generator",
			),
		}, nil
	}
	child, exception, err := newFunctionFrame(frame, instruction, evaluator, nil, nil)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	child.typeVar = &typeVarLoad{
		variable:    variable,
		kind:        kind,
		instruction: instruction,
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

func finishTypeVarLoad(load *typeVarLoad, value Value) Value {
	switch load.kind {
	case typeVarBoundLoad:
		load.variable.bound = value
		load.variable.boundEvaluated = true
	case typeVarConstraintsLoad:
		load.variable.constraints = value
		load.variable.constraintsEvaluated = true
	case typeVarDefaultLoad:
		load.variable.defaultValue = value
		load.variable.defaultEvaluated = true
	}
	return value
}

// executeSetTypeAliasParameters attaches compiler-created TypeVars and keeps
// the alias on the stack for binding or return.
func executeSetTypeAliasParameters(frame *frame, instruction int) (instructionOutcome, error) {
	aliasValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	parametersValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	alias, ok := aliasValue.(*typeAliasValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"type alias parameter target is not a type alias",
		)
	}
	parameters, ok := parametersValue.(*tupleValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"type alias parameters payload is not a tuple",
		)
	}
	for _, parameter := range parameters.elements {
		switch parameter.(type) {
		case *typeVarValue, *typeVarTupleValue, *paramSpecValue:
		default:
			return instructionOutcome{}, frame.failure(
				instruction,
				"type alias parameter payload contains a non-type-parameter value",
			)
		}
	}
	alias.typeParams = parameters
	return pushOutcome(frame, instruction, alias)
}

// executeMakeTypeAlias validates the compiler-created name and value function,
// captures module metadata, and pushes an unevaluated alias object.
func executeMakeTypeAlias(frame *frame, instruction int) (instructionOutcome, error) {
	computeValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	nameValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	compute, ok := computeValue.(*functionValue)
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "type alias value payload is not a function")
	}
	name, ok := nameValue.(*stringValue)
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "type alias name is not a string")
	}
	module := None
	if value, found := compute.globals.get("__name__"); found {
		module = value
	}
	return pushOutcome(frame, instruction, &typeAliasValue{
		name:       name.value,
		module:     module,
		typeParams: &tupleValue{},
		compute:    compute,
	})
}

// executeTypeAliasValueLoad returns a cached value or starts the hidden value
// function through the ordinary frame loop.
func executeTypeAliasValueLoad(
	frame *frame,
	instruction int,
	alias *typeAliasValue,
) (instructionOutcome, error) {
	if alias.evaluated {
		return pushOutcome(frame, instruction, alias.value)
	}
	if alias.compute.code.code.Flags()&bytecode.Generator != 0 {
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "type alias value function returned a generator"),
		}, nil
	}
	child, exception, err := newFunctionFrame(
		frame,
		instruction,
		alias.compute,
		nil,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	child.typeAlias = &typeAliasLoad{alias: alias, instruction: instruction}
	return instructionOutcome{kind: called, frame: child}, nil
}
