package runtime

import (
	"strconv"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

const functoolsModuleName = "_functools"

type cmpKeyValue struct {
	comparator Value
	object     Value
}

func (*cmpKeyValue) TypeName() string { return "functools.KeyWrapper" }
func (*cmpKeyValue) Repr() string     { return "<functools.KeyWrapper object>" }
func (*cmpKeyValue) isValue()         {}

type cmpKeyComparisonCall struct {
	instruction int
	operand     uint32
	sorting     *sortCall
}

func newFunctoolsModule() *Module {
	globals := newNamespace()
	globals.values["__name__"] = &stringValue{value: functoolsModuleName}
	globals.values["__package__"] = &stringValue{value: ""}
	globals.values["cmp_to_key"] = &builtinFunctionValue{
		name: "cmp_to_key",
		call: builtinCmpToKey,
	}
	return &Module{name: functoolsModuleName, globals: globals}
}

func builtinCmpToKey(arguments []Value, keywords *dictValue) (Value, *Exception) {
	comparator, exception := bindSingleNamedArgument(
		"cmp_to_key",
		"mycmp",
		arguments,
		keywords,
	)
	if exception != nil {
		return nil, exception
	}
	return &cmpKeyValue{comparator: comparator}, nil
}

func executeCmpKeyCall(
	caller *frame,
	instruction int,
	base int,
	key *cmpKeyValue,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	object, exception := bindSingleNamedArgument("K", "obj", arguments, keywords)
	discardCallSegment(caller, base)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return pushOutcome(caller, instruction, &cmpKeyValue{
		comparator: key.comparator,
		object:     object,
	})
}

// bindSingleNamedArgument implements the Argument Clinic shape shared by
// cmp_to_key(mycmp) and its returned K(obj) wrapper.
func bindSingleNamedArgument(
	callName string,
	argumentName string,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception) {
	if len(arguments) > 1 {
		return nil, newException(
			"TypeError",
			callName+"() takes at most 1 argument ("+
				strconv.Itoa(len(arguments))+" given)",
		)
	}
	var value Value
	if len(arguments) == 1 {
		value = arguments[0]
	}
	if keywords != nil {
		for _, entry := range keywords.entries {
			name := entry.key.(*stringValue).value
			if name != argumentName {
				return nil, newException(
					"TypeError",
					callName+"() got an unexpected keyword argument '"+name+"'",
				)
			}
			if value != nil {
				return nil, newException(
					"TypeError",
					callName+"() got multiple values for argument '"+argumentName+"'",
				)
			}
			value = entry.value
		}
	}
	if value == nil {
		return nil, newException(
			"TypeError",
			callName+"() missing required argument '"+argumentName+"' (pos 1)",
		)
	}
	return value, nil
}

func executeCmpKeyAttributeLoad(
	frame *frame,
	instruction int,
	key *cmpKeyValue,
	name string,
) (instructionOutcome, error) {
	if name == "obj" && key.object != nil {
		return pushOutcome(frame, instruction, key.object)
	}
	return raiseOutcome(newException("AttributeError", "object")), nil
}

// executeCmpKeyComparison validates both wrappers and calls the left wrapper's
// comparator with their retained objects.
func executeCmpKeyComparison(
	frame *frame,
	instruction int,
	operand uint32,
	left Value,
	right Value,
	sorting *sortCall,
) (instructionOutcome, error) {
	leftKey, leftOK := left.(*cmpKeyValue)
	rightKey, rightOK := right.(*cmpKeyValue)
	if !leftOK || !rightOK {
		return raiseOutcome(newException(
			"TypeError",
			"other argument must be K instance",
		)), nil
	}
	if leftKey.object == nil || rightKey.object == nil {
		return raiseOutcome(newException("AttributeError", "object")), nil
	}
	call := &cmpKeyComparisonCall{
		instruction: instruction,
		operand:     operand,
		sorting:     sorting,
	}
	outcome, err := executeFunctionCall(
		frame,
		instruction,
		len(frame.stack),
		leftKey.comparator,
		[]Value{leftKey.object, rightKey.object},
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.cmpKeyComparison = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"cmp_to_key comparator returned without a value",
		)
	}
	return finishCmpKeyComparator(frame, call, result)
}

// finishCmpKeyComparator compares one user comparator result with zero, routing
// user-defined result types through ordinary rich comparison.
func finishCmpKeyComparator(
	frame *frame,
	call *cmpKeyComparisonCall,
	result Value,
) (instructionOutcome, error) {
	zero := Value(&intValue{})
	_, resultUser := result.(*instanceValue)
	if resultUser {
		var comparison *comparisonCall
		if call.operand == bytecode.CompareEqual ||
			call.operand == bytecode.CompareNotEqual {
			comparison = newEqualityCall(
				call.instruction,
				call.operand,
				result,
				zero,
			)
		} else {
			comparison = newOrderingCall(
				call.instruction,
				call.operand,
				result,
				zero,
			)
		}
		comparison.sorting = call.sorting
		return continueComparisonCall(frame, comparison)
	}

	matched, exception := compareCmpKeyResult(result, zero, call.operand)
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	if call.sorting != nil {
		finishSortInsertionStep(call.sorting, matched)
		return continueSortInsertion(frame, call.sorting)
	}
	return pushOutcome(frame, call.instruction, booleanValue(matched))
}

// compareCmpKeyResult applies one fixed comparison between a comparator result
// and zero, preserving Python's unsupported-ordering errors.
func compareCmpKeyResult(left, right Value, operand uint32) (bool, *Exception) {
	if operand == bytecode.CompareEqual {
		return valuesEqual(left, right), nil
	}
	if operand == bytecode.CompareNotEqual {
		return !valuesEqual(left, right), nil
	}
	comparison, ordered, supported := orderedValues(left, right)
	if !supported {
		operator := orderingOperator(operand)
		return false, newException(
			"TypeError",
			"'"+operator+"' not supported between instances of '"+
				left.TypeName()+"' and '"+right.TypeName()+"'",
		)
	}
	if !ordered {
		return false, nil
	}
	switch operand {
	case bytecode.CompareLess:
		return comparison < 0, nil
	case bytecode.CompareLessEqual:
		return comparison <= 0, nil
	case bytecode.CompareGreater:
		return comparison > 0, nil
	default:
		return comparison >= 0, nil
	}
}

func orderingOperator(operand uint32) string {
	switch operand {
	case bytecode.CompareLess:
		return "<"
	case bytecode.CompareLessEqual:
		return "<="
	case bytecode.CompareGreater:
		return ">"
	default:
		return ">="
	}
}

var _ Value = (*cmpKeyValue)(nil)
