package runtime

import "strconv"

type dictionaryInputCall struct {
	instruction int
	target      *dictValue
	source      Value
	iterator    Value
	sentinel    Value
	keys        []Value
	keywords    []dictEntry
	mapping     bool
	position    int
	done        bool
}

// start prefers native dictionary copies, then a real keys attribute, then
// iterable pairs. Only AttributeError from keys lookup permits that fallback.
func (call *dictionaryInputCall) start(caller *frame) (instructionOutcome, error) {
	if call.source == nil {
		return call.finish(caller)
	}
	if source, ok := fastDictionarySource(caller.runtime, call.source); ok {
		for _, entry := range append([]dictEntry(nil), source.entries...) {
			if exception := call.target.set(entry.key, entry.value); exception != nil {
				return raiseOutcome(exception), nil
			}
		}
		return call.finish(caller)
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeDynamicAttributeLoad(caller, call.instruction, call.source, "keys")
	}, func(current *frame, method Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if !isAttributeError(exception) {
				return raiseOutcome(exception), nil
			}
			return call.startPairs(current)
		}
		return continueNativeOperation(current, call.instruction, func() (instructionOutcome, error) {
			return executeFunctionCall(current, call.instruction, len(current.stack), method, nil, nil)
		}, func(ready *frame, keys Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return call.captureKeys(ready, keys)
		})
	})
}

// fastDictionarySource follows the native dictionary-copy rule: a dict subtype
// qualifies only while it inherits the native iteration slot unchanged.
func fastDictionarySource(runtime *Runtime, source Value) (*dictValue, bool) {
	switch source := source.(type) {
	case *dictValue:
		return source, true
	case *mappingProxyValue:
		return source.dictionary, true
	case *instanceValue:
		if source.dictionary != nil {
			method, _ := source.class.lookup("__iter__")
			native, _, _ := runtime.nativeNamespace(dictNativeType).get(&stringValue{value: "__iter__"})
			if method == native {
				return source.dictionary, true
			}
		}
	}
	return nil, false
}

func (call *dictionaryInputCall) startPairs(caller *frame) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeIteratorLookup(caller, call.instruction, call.source)
	}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call.iterator = iterator
		return call.advance(current)
	})
}

// captureKeys drains keys before the first value lookup, preserving mutations
// and exceptions in Python iterators without iterating a live view afterward.
func (call *dictionaryInputCall) captureKeys(caller *frame, keys Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return startCollectionConstructor(caller, &collectionConstructorCall{instruction: call.instruction, kind: collectionTuple, iterable: keys})
	}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		call.mapping = true
		call.keys = result.(*tupleValue).elements
		return call.advance(current)
	})
}

// advance inserts each complete pair before requesting another. Immediate
// native steps use a trampoline; Python callbacks suspend in heap VM frames.
func (call *dictionaryInputCall) advance(caller *frame) (instructionOutcome, error) {
	for !call.done {
		suspended := false
		outcome, err := continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			outcome, err := call.nextPair(caller)
			suspended = outcome.kind == called
			return outcome, err
		}, func(current *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if result == call.sentinel {
				call.done = true
			} else {
				pair := result.(*tupleValue).elements
				if exception := call.target.set(pair[0], pair[1]); exception != nil {
					return raiseOutcome(exception), nil
				}
				call.position++
			}
			if suspended {
				return call.advance(current)
			}
			return pushOutcome(current, call.instruction, None)
		})
		if err != nil || outcome.kind != advance {
			return outcome, err
		}
		caller.pop()
	}
	return call.finish(caller)
}

// nextPair reads a captured mapping key or one outer iterable item, keeping
// source exhaustion separate from failures inside value or pair conversion.
func (call *dictionaryInputCall) nextPair(caller *frame) (instructionOutcome, error) {
	if call.mapping {
		if call.position == len(call.keys) {
			return pushOutcome(caller, call.instruction, call.sentinel)
		}
		key := call.keys[call.position]
		return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
			return executeSubscriptValue(caller, call.instruction, call.source, key)
		}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			return pushOutcome(current, call.instruction, &tupleValue{elements: []Value{key, value}})
		})
	}
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeBuiltinNext(caller, call.instruction, len(caller.stack), []Value{call.iterator, call.sentinel}, nil)
	}, func(current *frame, value Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		if value == call.sentinel {
			return pushOutcome(current, call.instruction, value)
		}
		return call.convertPair(current, value)
	})
}

// convertPair translates only failures obtaining an iterator to the sequence
// conversion error. Errors while consuming that iterator keep their own type.
func (call *dictionaryInputCall) convertPair(caller *frame, value Value) (instructionOutcome, error) {
	return continueNativeOperation(caller, call.instruction, func() (instructionOutcome, error) {
		return executeIteratorLookup(caller, call.instruction, value)
	}, func(current *frame, iterator Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			if exception.class.isSubclassOf(typeErrorType) {
				exception = newException("TypeError", "cannot convert dictionary update sequence element #"+strconv.Itoa(call.position)+" to a sequence")
			}
			return raiseOutcome(exception), nil
		}
		return continueNativeOperation(current, call.instruction, func() (instructionOutcome, error) {
			return continueCollectionConstructor(current, &collectionConstructorCall{instruction: call.instruction, kind: collectionTuple, iterator: iterator})
		}, func(ready *frame, result Value, exception *Exception) (instructionOutcome, error) {
			if exception != nil {
				return raiseOutcome(exception), nil
			}
			if count := len(result.(*tupleValue).elements); count != 2 {
				return raiseOutcome(newException("ValueError", "dictionary update sequence element #"+strconv.Itoa(call.position)+" has length "+strconv.Itoa(count)+"; 2 is required")), nil
			}
			return pushOutcome(ready, call.instruction, result)
		})
	})
}

func (call *dictionaryInputCall) finish(caller *frame) (instructionOutcome, error) {
	for _, entry := range call.keywords {
		if exception := call.target.set(entry.key, entry.value); exception != nil {
			return raiseOutcome(exception), nil
		}
	}
	return pushOutcome(caller, call.instruction, None)
}

func executeDictionaryConstructor(caller *frame, instruction, base int, arguments []Value, keywords *dictValue) (instructionOutcome, error) {
	if len(arguments) > 1 {
		discardCallSegment(caller, base)
		return raiseOutcome(newException("TypeError", "dict expected at most 1 argument, got "+strconv.Itoa(len(arguments)))), nil
	}
	arguments = append([]Value(nil), arguments...)
	discardCallSegment(caller, base)
	result := &dictValue{}
	return continueNativeOperation(caller, instruction, func() (instructionOutcome, error) {
		return executeDictionaryUpdateCall(caller, instruction, len(caller.stack), &dictionaryUpdateMethod{dictionary: result}, arguments, keywords)
	}, func(current *frame, _ Value, exception *Exception) (instructionOutcome, error) {
		if exception != nil {
			return raiseOutcome(exception), nil
		}
		return pushOutcome(current, instruction, result)
	})
}
