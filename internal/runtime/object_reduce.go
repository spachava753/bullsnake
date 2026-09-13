package runtime

// executeObjectReduce invokes the unchanged protocol-zero helper without
// redispatching to an override, allowing overrides to delegate to object safely.
func executeObjectReduce(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__reduce__", arguments, keywords, 0, 0); exception != nil {
		return raiseOutcome(exception), nil
	}
	return executeCommonReduction(caller, instruction, self, integerFromInt64(0))
}

// executeObjectReduceEx converts the protocol before resolving the actual
// __reduce__ attribute. Instance replacements participate only for class overrides.
func executeObjectReduceEx(caller *frame, instruction int, self Value, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if exception := checkNativeArguments("__reduce_ex__", arguments, keywords, 1, 1); exception != nil {
		return raiseOutcome(exception), nil
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeIOIndex(caller, instruction, arguments[0])
	}, func(current *frame, protocol Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		number := protocol.(*intValue)
		if number.value.Int64() < -2147483648 || number.value.Int64() > 2147483647 {
			return raiseOutcome(newException("OverflowError", "Python int too large to convert to C int")), nil
		}
		return executeReductionOverride(current, instruction, self, number)
	})
}

// executeReductionOverride compares class lookup with the cached root descriptor
// by identity, preserving descriptor failures and instance-only override rules.
func executeReductionOverride(caller *frame, instruction int, self Value, protocol *intValue) (instructionOutcome, error) {
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, instruction, self, "__reduce__")
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if isAttributeError(exception) {
				return executeCommonReduction(current, instruction, self, protocol)
			}
			return raiseOutcome(exception), nil
		}
		class, _ := typeOf(self)
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeDynamicAttributeLoad(current, instruction, class, "__reduce__")
		}, func(current *frame, classMethod Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			root, _ := current.runtime.nativeClassAttribute(objectNativeType, "__reduce__")
			if classMethod != root {
				return executeFunctionCall(current, instruction, len(current.stack), method, nil, nil)
			}
			return executeCommonReduction(current, instruction, self, protocol)
		})
	})
}

// executeCommonReduction keeps unknown native layouts behind an explicit guard,
// then uses copyreg for old protocols or obtains genuine new-object arguments.
func executeCommonReduction(caller *frame, instruction int, self Value, protocol *intValue) (instructionOutcome, error) {
	if !supportsDefaultReduction(self) {
		return raiseOutcome(newException("TypeError", "default reduction for '"+self.TypeName()+"' is not implemented")), nil
	}
	if protocol.value.Int64() >= 2 {
		return executeNewObjectArguments(caller, instruction, self)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeNativeImport(caller, instruction, newImportRequest("copyreg", "copyreg", &tupleValue{}))
	}, func(current *frame, module Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return executeMethodCall(current, instruction, module, "_reduce_ex", []Value{self, protocol})
	})
}

func supportsDefaultReduction(self Value) bool {
	if instance, ok := self.(*instanceValue); ok {
		return rootAllocatableClass(instance.class) || instance.dictionary != nil
	}
	class, _ := typeOf(self)
	return class == objectNativeType || class == dictNativeType
}

// executeNewObjectReduction imports the real reconstruction callable, captures
// its argument tuple, and only then asks the object for state and mapping items.
func executeNewObjectReduction(caller *frame, instruction int, self Value, args *tupleValue, kwargs Value) (instructionOutcome, error) {
	var newargs *tupleValue
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeNativeImport(caller, instruction, newImportRequest("copyreg", "copyreg", &tupleValue{}))
		}, func(current *frame, module Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			class, _ := typeOf(self)
			name := "__newobj__"
			newargs = &tupleValue{elements: append([]Value{class}, args.elements...)}
			if dictionary, ok := dictionaryStorage(kwargs); ok && len(dictionary.entries) != 0 {
				name = "__newobj_ex__"
				newargs = &tupleValue{elements: []Value{class, args, kwargs}}
			}
			return executeDynamicAttributeLoad(current, instruction, module, name)
		})
	}, func(current *frame, constructor Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(current, instruction, self, "__getstate__", nil)
		}, func(current *frame, state Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			result := &tupleValue{elements: []Value{constructor, newargs, state, None, None}}
			return finishReductionItems(current, instruction, self, result)
		})
	})
}

// finishReductionItems obtains the object's actual items method and iterator,
// retaining lazy contents and propagating errors after state has been obtained.
func finishReductionItems(caller *frame, instruction int, self Value, result *tupleValue) (instructionOutcome, error) {
	if _, dictionary := dictionaryStorage(self); !dictionary {
		return pushOutcome(caller, instruction, result)
	}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
			return executeMethodCall(caller, instruction, self, "items", nil)
		}, func(current *frame, items Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return executeIteratorLookup(current, instruction, items)
		})
	}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		result.elements[4] = iterator
		return pushOutcome(current, instruction, result)
	})
}
