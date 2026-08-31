package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

type typeVarValue struct {
	name                 string
	boundEvaluator       *functionValue
	bound                Value
	boundEvaluated       bool
	constraintsEvaluator *functionValue
	constraints          Value
	constraintsEvaluated bool
}

func (*typeVarValue) TypeName() string { return "typing.TypeVar" }
func (variable *typeVarValue) Repr() string {
	return variable.name
}
func (*typeVarValue) isValue() {}

type typeVarLoad struct {
	variable    *typeVarValue
	constraints bool
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

// executeMakeTypeVar creates one inferred-variance PEP 695 type variable.
func executeMakeTypeVar(frame *frame, instruction int) (instructionOutcome, error) {
	nameValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	name, ok := nameValue.(*stringValue)
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "type variable name is not a string")
	}
	return pushOutcome(frame, instruction, &typeVarValue{name: name.value})
}

// executeSetTypeVarEvaluator attaches one compiler-created lazy evaluator and
// keeps the TypeVar on the stack for its hidden generic scope.
func executeSetTypeVarEvaluator(
	frame *frame,
	instruction int,
	constraints bool,
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
	if constraints {
		variable.constraintsEvaluator = evaluator
	} else {
		variable.boundEvaluator = evaluator
	}
	return pushOutcome(frame, instruction, variable)
}

// executeTypeVarLoad returns a cached bound or constraints value, or starts its
// hidden evaluator through the ordinary frame loop.
func executeTypeVarLoad(
	frame *frame,
	instruction int,
	variable *typeVarValue,
	constraints bool,
) (instructionOutcome, error) {
	evaluator := variable.boundEvaluator
	if constraints {
		if variable.constraintsEvaluated {
			return pushOutcome(frame, instruction, variable.constraints)
		}
		evaluator = variable.constraintsEvaluator
	} else if variable.boundEvaluated {
		return pushOutcome(frame, instruction, variable.bound)
	}
	if evaluator == nil {
		if constraints {
			return pushOutcome(frame, instruction, &tupleValue{})
		}
		return pushOutcome(frame, instruction, None)
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
		constraints: constraints,
		instruction: instruction,
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

func finishTypeVarLoad(load *typeVarLoad, value Value) Value {
	if load.constraints {
		load.variable.constraints = value
		load.variable.constraintsEvaluated = true
	} else {
		load.variable.bound = value
		load.variable.boundEvaluated = true
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
		if _, ok := parameter.(*typeVarValue); !ok {
			return instructionOutcome{}, frame.failure(
				instruction,
				"type alias parameter payload contains a non-TypeVar value",
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
