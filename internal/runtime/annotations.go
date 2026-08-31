package runtime

import "fmt"

type namespaceValue struct {
	namespace *Namespace
}

func (*namespaceValue) TypeName() string { return "dict" }
func (*namespaceValue) Repr() string     { return "<class namespace>" }
func (*namespaceValue) isValue()         {}

func executeLoadFromDictOrGlobals(
	frame *frame,
	instruction int,
	name string,
) (instructionOutcome, error) {
	namespace, err := popAnnotationNamespace(frame, instruction)
	if err != nil {
		return instructionOutcome{}, err
	}
	value, found := namespace.get(name)
	if !found {
		value, found = frame.globals.get(name)
	}
	if !found {
		value, found = frame.builtins.get(name)
	}
	if !found {
		return instructionOutcome{
			kind:      raised,
			exception: newException("NameError", fmt.Sprintf("name '%s' is not defined", name)),
		}, nil
	}
	return pushOutcome(frame, instruction, value)
}

func executeLoadFromDictOrDeref(
	frame *frame,
	instruction int,
	derefIndex int,
) (instructionOutcome, error) {
	namespace, err := popAnnotationNamespace(frame, instruction)
	if err != nil {
		return instructionOutcome{}, err
	}
	name := derefName(frame.code, derefIndex)
	value, found := namespace.get(name)
	if !found {
		value = frame.deref[derefIndex].value
		if value == nil {
			return instructionOutcome{
				kind:      raised,
				exception: unboundDerefException(frame.code, derefIndex),
			}, nil
		}
	}
	return pushOutcome(frame, instruction, value)
}

func popAnnotationNamespace(
	frame *frame,
	instruction int,
) (*Namespace, error) {
	value, ok := frame.pop()
	if !ok {
		return nil, frame.failure(instruction, "operand stack underflow")
	}
	namespace, ok := value.(*namespaceValue)
	if !ok || namespace.namespace == nil {
		return nil, frame.failure(
			instruction,
			"class annotation namespace is not a namespace",
		)
	}
	return namespace.namespace, nil
}
