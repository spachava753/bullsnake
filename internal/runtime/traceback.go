package runtime

// tracebackValue is the canonical outermost-first exception chain. Nodes keep
// actual frames and event instruction positions, not copied frame snapshots.
type tracebackValue struct {
	frame       *frame
	instruction int
	next        *tracebackValue
}

func (*tracebackValue) TypeName() string { return "traceback" }
func (*tracebackValue) Repr() string     { return "<traceback object>" }
func (*tracebackValue) isValue()         {}

// setTraceback shares a supplied chain or clears it. Resetting the private
// origin prevents a cleared chain from retaining its old origin frame.
func (exception *Exception) setTraceback(value Value) *Exception {
	if value == nil {
		return newException("TypeError", "__traceback__ may not be deleted")
	}
	var traceback *tracebackValue
	if value != None {
		var ok bool
		traceback, ok = value.(*tracebackValue)
		if !ok {
			return newException("TypeError", "__traceback__ must be a traceback or None")
		}
	}
	exception.traceback = traceback
	exception.originFrame = nil
	exception.originInstruction = 0
	return nil
}

// executeTracebackAttributeLoad exposes stable frame identity and the event's
// saved instruction/line, with None marking the end of the shared chain.
func executeTracebackAttributeLoad(caller *frame, instruction int, traceback *tracebackValue, name string) (instructionOutcome, error) {
	switch name {
	case "tb_frame":
		return pushOutcome(caller, instruction, traceback.frame.pythonFrame())
	case "tb_next":
		if traceback.next == nil {
			return pushOutcome(caller, instruction, None)
		}
		return pushOutcome(caller, instruction, traceback.next)
	case "tb_lasti":
		return pushOutcome(caller, instruction, integerFromInt64(int64(traceback.instruction)))
	case "tb_lineno":
		position := traceback.frame.position(traceback.instruction)
		return pushOutcome(caller, instruction, integerFromInt64(int64(position.Start.Line)))
	}
	return raiseOutcome(newException("AttributeError", "'traceback' object has no attribute '"+name+"'")), nil
}

// storeTracebackNext allows acyclic chain splicing. Validation completes before
// mutation, so a type error or detected loop leaves the old link unchanged.
func storeTracebackNext(traceback *tracebackValue, value Value) *Exception {
	if value == nil {
		return newException("TypeError", "can't delete tb_next attribute")
	}
	var next *tracebackValue
	if value != None {
		var ok bool
		next, ok = value.(*tracebackValue)
		if !ok {
			return newException("TypeError", "expected traceback object, got '"+value.TypeName()+"'")
		}
	}
	for cursor := next; cursor != nil; cursor = cursor.next {
		if cursor == traceback {
			return newException("ValueError", "traceback loop detected")
		}
	}
	traceback.next = next
	return nil
}

// executeTracebackStore keeps exception-chain and traceback-link mutation on
// the ordinary attribute path while leaving all other metadata read-only.
func executeTracebackStore(owner Value, name string, value Value) (instructionOutcome, error) {
	var exception *Exception
	switch owner := owner.(type) {
	case *Exception:
		if name == "__traceback__" {
			exception = owner.setTraceback(value)
		} else {
			exception = newException("AttributeError", "'"+owner.TypeName()+"' object has no attribute '"+name+"'")
		}
	case *tracebackValue:
		if name == "tb_next" {
			exception = storeTracebackNext(owner, value)
		} else {
			exception = newException("AttributeError", "readonly attribute")
		}
	}
	if exception != nil {
		return raiseOutcome(exception), nil
	}
	return instructionOutcome{kind: advance}, nil
}
