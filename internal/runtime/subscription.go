package runtime

type subscriptionCallKind uint8

const (
	subscriptionGet subscriptionCallKind = iota
	subscriptionSet
	subscriptionDelete
)

type subscriptionCall struct {
	kind        subscriptionCallKind
	instruction int
}

// executeUserSubscription resolves one class item method, invokes it through the
// frame loop, and records whether the return value is kept or discarded.
func executeUserSubscription(
	frame *frame,
	instruction int,
	kind subscriptionCallKind,
	instance *instanceValue,
	arguments []Value,
) (instructionOutcome, error) {
	name := "__getitem__"
	switch kind {
	case subscriptionSet:
		name = "__setitem__"
	case subscriptionDelete:
		name = "__delitem__"
	}
	method, found := lookupInstanceSpecial(instance, name)
	if !found {
		return instructionOutcome{
			kind:      raised,
			exception: missingSubscriptionMethod(instance, kind),
		}, nil
	}

	call := &subscriptionCall{kind: kind, instruction: instruction}
	outcome, err := executeFunctionCall(
		frame,
		instruction,
		len(frame.stack),
		method,
		arguments,
		nil,
	)
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == called {
		outcome.frame.subscription = call
		return outcome, nil
	}
	if outcome.kind != advance {
		return outcome, nil
	}
	result, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"subscription special method returned without a value",
		)
	}
	return finishSubscriptionCall(frame, call, result)
}

func finishSubscriptionCall(
	frame *frame,
	call *subscriptionCall,
	result Value,
) (instructionOutcome, error) {
	if call.kind == subscriptionGet {
		return pushOutcome(frame, call.instruction, result)
	}
	return instructionOutcome{kind: advance}, nil
}

func missingSubscriptionMethod(
	instance *instanceValue,
	kind subscriptionCallKind,
) *Exception {
	message := "'" + instance.TypeName() + "' object is not subscriptable"
	if kind == subscriptionSet {
		message = "'" + instance.TypeName() + "' object does not support item assignment"
	} else if kind == subscriptionDelete {
		message = "'" + instance.TypeName() + "' object does not support item deletion"
	}
	return newException("TypeError", message)
}
