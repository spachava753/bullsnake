package runtime

import (
	"strconv"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type sortItem struct {
	value Value
	key   Value
}

type sortCall struct {
	instruction   int
	key           Value
	reverseValue  Value
	reverse       bool
	values        []Value
	items         []sortItem
	keyIndex      int
	position      int
	scan          int
	current       sortItem
	inserting     bool
	target        *listValue
	abstractClass *typeValue
}

// executeBuiltinSorted validates the keyword-only controls, then collects the
// source through the ordinary iterator continuation before sorting it.
func executeBuiltinSorted(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	iterable, key, reverse, exception := bindSortedArguments(arguments, keywords)
	if exception != nil {
		discardCallSegment(caller, base)
		return raiseOutcome(exception), nil
	}
	call := &sortCall{
		instruction:  instruction,
		key:          key,
		reverseValue: reverse,
	}
	discardCallSegment(caller, base)
	return startCollectionConstructor(caller, &collectionConstructorCall{
		instruction: instruction,
		kind:        collectionSorted,
		iterable:    iterable,
		sorting:     call,
	})
}

// bindSortedArguments accepts one positional iterable plus the key and reverse
// keyword-only controls, reporting list.sort-style keyword errors.
func bindSortedArguments(
	arguments []Value,
	keywords *dictValue,
) (Value, Value, Value, *Exception) {
	if len(arguments) != 1 {
		return nil, nil, nil, newException(
			"TypeError",
			"sorted expected 1 argument, got "+strconv.Itoa(len(arguments)),
		)
	}
	key, reverse, exception := bindSortControls(keywords)
	if exception != nil {
		return nil, nil, nil, exception
	}
	return arguments[0], key, reverse, nil
}

// bindSortControls accepts the shared keyword-only controls for sorted and
// list.sort, retaining Python's list.sort name in unknown-keyword errors.
func bindSortControls(keywords *dictValue) (Value, Value, *Exception) {
	key := Value(None)
	reverse := Value(falseSingleton)
	if keywords != nil {
		for _, entry := range keywords.entries {
			name := entry.key.(*stringValue).value
			switch name {
			case "key":
				key = entry.value
			case "reverse":
				reverse = entry.value
			default:
				return nil, nil, newException(
					"TypeError",
					"'"+name+"' is an invalid keyword argument for sort()",
				)
			}
		}
	}
	return key, reverse, nil
}

func startSortValues(
	frame *frame,
	call *sortCall,
	values []Value,
) (instructionOutcome, error) {
	if call == nil {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"sort has no continuation state",
		)
	}
	call.values = values
	return executeTruthWithCall(frame, call.reverseValue, &truthCall{
		instruction: call.instruction,
		sortReverse: call,
	})
}

func finishSortReverse(
	frame *frame,
	call *sortCall,
	reverse bool,
) (instructionOutcome, error) {
	call.reverse = reverse
	call.items = make([]sortItem, len(call.values))
	if call.key == None {
		for index, value := range call.values {
			call.items[index] = sortItem{value: value, key: value}
		}
		return startSortInsertion(frame, call)
	}
	return continueSortKeys(frame, call)
}

// continueSortKeys evaluates one key per source item in original order and
// retains the continuation on a Python key-function frame when needed.
func continueSortKeys(frame *frame, call *sortCall) (instructionOutcome, error) {
	if call.keyIndex >= len(call.values) {
		return startSortInsertion(frame, call)
	}
	outcome, err := executeFunctionCall(
		frame,
		call.instruction,
		len(frame.stack),
		call.key,
		[]Value{call.values[call.keyIndex]},
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.sorting = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			call.instruction,
			"sort key returned without a value",
		)
	}
	return finishSortKey(frame, call, result)
}

func finishSortKey(
	frame *frame,
	call *sortCall,
	key Value,
) (instructionOutcome, error) {
	if call == nil || call.keyIndex >= len(call.values) {
		return instructionOutcome{}, frame.failure(
			frame.instruction-1,
			"sort key has invalid continuation state",
		)
	}
	call.items[call.keyIndex] = sortItem{
		value: call.values[call.keyIndex],
		key:   key,
	}
	call.keyIndex++
	return continueSortKeys(frame, call)
}

func startSortInsertion(
	frame *frame,
	call *sortCall,
) (instructionOutcome, error) {
	call.values = nil
	call.position = 1
	return continueSortInsertion(frame, call)
}

// continueSortInsertion performs a stable insertion sort and suspends only for
// user ordering or truth methods. Native comparisons stay in this loop.
func continueSortInsertion(
	frame *frame,
	call *sortCall,
) (instructionOutcome, error) {
	for call.position < len(call.items) {
		if !call.inserting {
			call.current = call.items[call.position]
			call.scan = call.position
			call.inserting = true
		}
		if call.scan == 0 {
			finishSortInsertionStep(call, false)
			continue
		}
		left := call.current.key
		right := call.items[call.scan-1].key
		if call.reverse {
			left, right = right, left
		}
		_, leftKey := left.(*cmpKeyValue)
		_, rightKey := right.(*cmpKeyValue)
		if leftKey || rightKey {
			return executeCmpKeyComparison(
				frame,
				call.instruction,
				bytecode.CompareLess,
				left,
				right,
				call,
			)
		}
		_, leftUser := left.(*instanceValue)
		_, rightUser := right.(*instanceValue)
		if leftUser || rightUser {
			comparison := newOrderingCall(
				call.instruction,
				bytecode.CompareLess,
				left,
				right,
			)
			comparison.sorting = call
			return continueComparisonCall(frame, comparison)
		}
		comparison, ordered, supported := orderedValues(left, right)
		if !supported {
			return raiseOutcome(newException(
				"TypeError",
				"'<' not supported between instances of '"+
					left.TypeName()+"' and '"+right.TypeName()+"'",
			)), nil
		}
		finishSortInsertionStep(call, ordered && comparison < 0)
	}
	if call.abstractClass != nil {
		return raiseOutcome(abstractAllocationError(call.abstractClass, call.items)), nil
	}
	values := make([]Value, len(call.items))
	for index, item := range call.items {
		values[index] = item.value
	}
	if call.target != nil {
		call.target.elements = values
		return pushOutcome(frame, call.instruction, None)
	}
	return pushOutcome(frame, call.instruction, &listValue{elements: values})
}

func finishSortInsertionStep(call *sortCall, shift bool) {
	if shift {
		call.items[call.scan] = call.items[call.scan-1]
		call.scan--
		return
	}
	call.items[call.scan] = call.current
	call.current = sortItem{}
	call.inserting = false
	call.position++
}
