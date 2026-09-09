package runtime

// nativeContinuation retains a native operation across Python calls. The
// caller resumes it only after existing child-frame protocols have finished.
type nativeContinuation struct {
	instruction int
	depth       int
	resume      func(*frame, Value, *Exception) (instructionOutcome, error)
	next        *nativeContinuation
}

// continueNativeOperation composes a result consumer with an operation that may
// suspend. Nested consumers resume from inner to outer on the same Python frame.
func continueNativeOperation(caller *frame, instruction int,
	operation func() (instructionOutcome, error),
	resume func(*frame, Value, *Exception) (instructionOutcome, error),
) (instructionOutcome, error) {
	previous := caller.nativeContinuation
	depth := len(caller.stack)
	outcome, err := operation()
	if err != nil {
		return instructionOutcome{}, err
	}
	if outcome.kind == raised {
		return resume(caller, nil, outcome.exception)
	}
	if outcome.kind == advance {
		result, ok := caller.pop()
		if !ok {
			return instructionOutcome{}, caller.failure(instruction, "native operation returned without a value")
		}
		return resume(caller, result, nil)
	}
	if outcome.kind != called {
		return outcome, nil
	}
	pending := &nativeContinuation{instruction: instruction, depth: depth, resume: resume, next: previous}
	if caller.nativeContinuation == previous {
		caller.nativeContinuation = pending
	} else {
		inner := caller.nativeContinuation
		for inner.next != previous {
			inner = inner.next
		}
		inner.next = pending
	}
	return outcome, nil
}
