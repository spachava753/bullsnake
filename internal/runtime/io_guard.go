package runtime

type ioGuard struct{ busy *bool }

func (guard *ioGuard) release() {
	if guard.busy != nil {
		*guard.busy = false
		guard.busy = nil
	}
}

// withIOGuard rejects reentrant buffer mutation. The frame also owns the guard
// so a Go failure releases it when Python continuation callbacks cannot run.
func withIOGuard(caller *frame, instruction int, busy *bool, operation func() (instructionOutcome, error)) (instructionOutcome, error) {
	if *busy {
		return raiseOutcome(newException("RuntimeError", "reentrant call inside buffered stream")), nil
	}
	*busy = true
	guard := &ioGuard{busy: busy}
	caller.ioGuards = append(caller.ioGuards, guard)
	return continueNativeOperation(caller, instruction, operation, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		guard.release()
		for len(current.ioGuards) > 0 && current.ioGuards[len(current.ioGuards)-1].busy == nil {
			current.ioGuards = current.ioGuards[:len(current.ioGuards)-1]
		}
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(current, instruction, value)
	})
}
