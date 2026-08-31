package runtime

import (
	"fmt"
	"math/big"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type namespaceValue struct {
	namespace *Namespace
}

func (*namespaceValue) TypeName() string { return "dict" }
func (*namespaceValue) Repr() string     { return "<class namespace>" }
func (*namespaceValue) isValue()         {}

type classAnnotationLoad struct {
	class       *typeValue
	instruction int
}

type functionAnnotationLoad struct {
	function    *functionValue
	instruction int
}

// executeFunctionAnnotationsLoad returns a cached dictionary or starts the
// function's annotation child so it runs through the ordinary frame loop.
func executeFunctionAnnotationsLoad(
	frame *frame,
	instruction int,
	function *functionValue,
) (instructionOutcome, error) {
	if function.annotations != nil {
		return pushOutcome(frame, instruction, function.annotations)
	}
	if function.annotate == nil {
		function.annotations = &dictValue{}
		return pushOutcome(frame, instruction, function.annotations)
	}
	if function.annotate.code.code.Flags()&bytecode.Generator != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"__annotate__ returned non-dict of type 'generator'",
			),
		}, nil
	}
	format := &intValue{value: *big.NewInt(1)}
	child, exception, err := newFunctionFrame(
		frame,
		instruction,
		function.annotate,
		[]Value{format},
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	child.functionAnnotations = &functionAnnotationLoad{
		function:    function,
		instruction: instruction,
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

func finishFunctionAnnotationsLoad(
	load *functionAnnotationLoad,
	value Value,
) (Value, *Exception) {
	annotations, ok := value.(*dictValue)
	if !ok {
		return nil, newException(
			"TypeError",
			"__annotate__ returned non-dict of type '"+value.TypeName()+"'",
		)
	}
	load.function.annotations = annotations
	return annotations, nil
}

// executeClassAnnotationsLoad returns an explicit or cached dictionary first;
// otherwise it starts the class's own annotation function and records the
// return-time cache work on that child frame.
func executeClassAnnotationsLoad(
	frame *frame,
	instruction int,
	class *typeValue,
) (instructionOutcome, error) {
	if annotations, found := class.namespace.get("__annotations__"); found {
		return pushOutcome(frame, instruction, annotations)
	}
	if annotations, found := class.namespace.get("__annotations_cache__"); found {
		return pushOutcome(frame, instruction, annotations)
	}

	annotate, found := class.namespace.get("__annotate__")
	if !found {
		annotate, found = class.namespace.get("__annotate_func__")
	}
	function, callable := annotate.(*functionValue)
	if !found || !callable {
		annotations := &dictValue{}
		class.namespace.values["__annotations_cache__"] = annotations
		return pushOutcome(frame, instruction, annotations)
	}
	if function.code.code.Flags()&bytecode.Generator != 0 {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"__annotate__ returned non-dict of type 'generator'",
			),
		}, nil
	}
	format := &intValue{value: *big.NewInt(1)}
	child, exception, err := newFunctionFrame(
		frame,
		instruction,
		function,
		[]Value{format},
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if exception != nil {
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	child.classAnnotations = &classAnnotationLoad{
		class:       class,
		instruction: instruction,
	}
	return instructionOutcome{kind: called, frame: child}, nil
}

func finishClassAnnotationsLoad(
	load *classAnnotationLoad,
	value Value,
) (Value, *Exception) {
	annotations, ok := value.(*dictValue)
	if !ok {
		return nil, newException(
			"TypeError",
			"__annotate__ returned non-dict of type '"+value.TypeName()+"'",
		)
	}
	load.class.namespace.values["__annotations_cache__"] = annotations
	return annotations, nil
}

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
