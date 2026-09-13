package runtime

// executeNativeImport loads an absolute request through the ordinary cache and
// loader. Module-frame completion signals resume the request without replaying
// the caller's instruction; ordinary nested imports keep their own requests.
func executeNativeImport(caller *frame, instruction int, request *importRequest) (instructionOutcome, error) {
	request.native = true
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return advanceImport(caller, instruction, request)
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == None {
			return executeNativeImport(current, instruction, request)
		}
		return pushOutcome(current, instruction, result)
	})
}
