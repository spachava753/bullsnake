package runtime

// executeGenericAliasEquality truth-tests origin equality before comparing the
// argument tuples, preserving Python callbacks and declining non-alias operands.
func executeGenericAliasEquality(caller *frame, instruction int, self *instanceValue, other Value, invert bool) (instructionOutcome, error) {
	if self.alias == nil {
		return raiseOutcome(newException("TypeError", "uninitialized GenericAlias")), nil
	}
	right, ok := other.(*instanceValue)
	if !ok || right.alias == nil {
		return pushOutcome(caller, instruction, notImplementedSingleton)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeEqualityValue(caller, instruction, self.alias.origin, right.alias.origin, nil)
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeBuiltinBool(current, instruction, len(current.stack), []Value{result}, nil)
		})
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if result == falseSingleton {
			return pushOutcome(current, instruction, booleanValue(invert))
		}
		return executeSequenceEquality(current, instruction, self.alias.args, right.alias.args, invert, nil)
	})
}

// executeGenericAliasHash combines origin and argument hashes with XOR. The
// alias does not cache its hash; its immutable argument tuple owns that cache.
func executeGenericAliasHash(caller *frame, instruction int, self *instanceValue, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__hash__", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	if self.alias == nil {
		return raiseOutcome(newException("TypeError", "uninitialized GenericAlias")), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeBuiltinHash(caller, instruction, len(caller.stack), []Value{self.alias.origin}, nil)
	}, func(current *frame, originHash Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeBuiltinHash(current, instruction, len(current.stack), []Value{self.alias.args}, nil)
		}, func(ready *frame, argsHash Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			hash := originHash.(*intValue).value.Int64() ^ argsHash.(*intValue).value.Int64()
			return pushOutcome(ready, instruction, hashIntegerValue(normalizeHashInt64(hash)))
		})
	})
}
