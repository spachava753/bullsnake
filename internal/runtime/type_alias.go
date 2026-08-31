package runtime

import "github.com/spachava753/bullsnake/internal/compiler/bytecode"

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
