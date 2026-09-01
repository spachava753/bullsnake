package runtime

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type outcomeKind uint8

const (
	advance outcomeKind = iota
	called
	returned
	yielded
	raised
)

type instructionOutcome struct {
	kind      outcomeKind
	value     Value
	exception *Exception
	frame     *frame
	reraise   bool
}

type raisedOutcome struct {
	exception   *Exception
	frame       *frame
	instruction int
}

// execute advances the active heap frame until it returns, raises a Python
// exception, or encounters a validated-bytecode invariant failure.
func execute(thread *threadState) (result Value, unhandled *raisedOutcome, err error) {
	defer func() {
		if err == nil {
			return
		}
		for current := thread.current; current != nil; current = current.previous {
			current.discardImportedModule()
		}
	}()
	for thread.current != nil {
		active := thread.current
		index := active.instruction
		if index < 0 || index >= len(active.code.instructions) {
			return nil, nil, active.failure(index, "instruction index out of range")
		}
		active.pruneHandledExceptions(index)
		instruction := active.code.instructions[index]
		active.instruction++
		outcome, err := executeInstruction(active, index, instruction)
		if err != nil {
			return nil, nil, err
		}
		switch outcome.kind {
		case advance:
			continue
		case called:
			if outcome.frame == nil || outcome.frame.previous != active {
				return nil, nil, active.failure(index, "invalid call frame transition")
			}
			thread.current = outcome.frame
			if outcome.exception != nil {
				injectedAt := outcome.frame.instruction - 1
				if injectedAt < 0 {
					return nil, nil, outcome.frame.failure(
						injectedAt,
						"injected exception has no suspended instruction",
					)
				}
				unhandled, routeErr := routeException(
					thread,
					outcome.frame,
					injectedAt,
					outcome.exception,
					false,
				)
				if routeErr != nil {
					return nil, nil, routeErr
				}
				if unhandled != nil {
					return nil, unhandled, nil
				}
			}
		case yielded:
			caller, suspensionException, suspendErr := suspendGenerator(
				active,
				index,
				outcome.value,
			)
			if suspendErr != nil {
				return nil, nil, suspendErr
			}
			thread.current = caller
			if suspensionException != nil {
				unhandled, routeErr := routeException(
					thread,
					caller,
					caller.instruction-1,
					suspensionException,
					false,
				)
				if routeErr != nil {
					return nil, nil, routeErr
				}
				if unhandled != nil {
					return nil, unhandled, nil
				}
			}
		case returned:
			if active.generator != nil {
				caller, completionException, finishErr := finishGenerator(
					active,
					index,
					outcome.value,
				)
				if finishErr != nil {
					return nil, nil, finishErr
				}
				thread.current = caller
				if completionException != nil {
					unhandled, routeErr := routeException(
						thread,
						caller,
						caller.instruction-1,
						completionException,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
				}
				continue
			}
			thread.current = active.previous
			result := outcome.value
			if active.classAnnotations != nil {
				load := active.classAnnotations
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"class annotation load has no caller",
					)
				}
				var annotationException *Exception
				result, annotationException = finishClassAnnotationsLoad(load, result)
				if annotationException != nil {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						load.instruction,
						annotationException,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
			}
			if active.functionAnnotations != nil {
				load := active.functionAnnotations
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"function annotation load has no caller",
					)
				}
				var annotationException *Exception
				result, annotationException = finishFunctionAnnotationsLoad(load, result)
				if annotationException != nil {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						load.instruction,
						annotationException,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
			}
			if active.typeVar != nil {
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"type variable evaluator has no caller",
					)
				}
				result = finishTypeVarLoad(active.typeVar, result)
			}
			if active.typeAlias != nil {
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"type alias value load has no caller",
					)
				}
				active.typeAlias.alias.value = result
				active.typeAlias.alias.evaluated = true
			}
			if active.instanceInit != nil {
				initialization := active.instanceInit
				if result != None {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						initialization.instruction,
						newException(
							"TypeError",
							"__init__() should return None, not '"+
								result.TypeName()+"'",
						),
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				result = initialization.instance
			}
			if active.classBuild != nil {
				build := active.classBuild
				var classException *Exception
				result, classException = build.finish(result)
				if classException != nil {
					if thread.current == nil {
						return nil, nil, active.failure(
							index,
							"class construction has no caller",
						)
					}
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						build.instruction,
						classException,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
			}
			if active.mapping != nil {
				call := active.mapping
				active.mapping = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"mapped callable has no caller",
					)
				}
				mapOutcome, mapErr := finishMapItem(thread.current, call, result)
				if mapErr != nil {
					return nil, nil, mapErr
				}
				if mapOutcome.kind == called {
					if mapOutcome.frame == nil ||
						mapOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.request.instruction,
							"invalid chained map frame transition",
						)
					}
					thread.current = mapOutcome.frame
					continue
				}
				if mapOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.request.instruction,
						mapOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if mapOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.request.instruction,
						"invalid mapped callable outcome",
					)
				}
				continue
			}
			if active.filtering != nil {
				call := active.filtering
				active.filtering = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"filter predicate has no caller",
					)
				}
				filterOutcome, filterErr := finishFilterPredicate(
					thread.current,
					call,
					result,
				)
				if filterErr != nil {
					return nil, nil, filterErr
				}
				if filterOutcome.kind == called {
					if filterOutcome.frame == nil ||
						filterOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.request.instruction,
							"invalid chained filter frame transition",
						)
					}
					thread.current = filterOutcome.frame
					continue
				}
				if filterOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.request.instruction,
						filterOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if filterOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.request.instruction,
						"invalid filter predicate outcome",
					)
				}
				continue
			}
			if active.sorting != nil {
				call := active.sorting
				active.sorting = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"sort key has no caller",
					)
				}
				sortOutcome, sortErr := finishSortKey(thread.current, call, result)
				if sortErr != nil {
					return nil, nil, sortErr
				}
				switch sortOutcome.kind {
				case advance:
					continue
				case called:
					if sortOutcome.frame == nil ||
						sortOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid chained sort frame transition",
						)
					}
					thread.current = sortOutcome.frame
					continue
				case raised:
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						sortOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				default:
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid sort key outcome",
					)
				}
			}
			if active.cmpKeyComparison != nil {
				call := active.cmpKeyComparison
				active.cmpKeyComparison = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"cmp_to_key comparator has no caller",
					)
				}
				comparisonOutcome, comparisonErr := finishCmpKeyComparator(
					thread.current,
					call,
					result,
				)
				if comparisonErr != nil {
					return nil, nil, comparisonErr
				}
				switch comparisonOutcome.kind {
				case advance:
					continue
				case called:
					if comparisonOutcome.frame == nil ||
						comparisonOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid cmp_to_key comparison frame transition",
						)
					}
					thread.current = comparisonOutcome.frame
					continue
				case raised:
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						comparisonOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				default:
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid cmp_to_key comparator outcome",
					)
				}
			}
			if active.truth != nil {
				call := active.truth
				active.truth = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"truth special method has no caller",
					)
				}
				truthOutcome, truthErr := finishTruthCall(thread.current, call, result)
				if truthErr != nil {
					return nil, nil, truthErr
				}
				if truthOutcome.kind == called {
					if truthOutcome.frame == nil ||
						truthOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid chained truth frame transition",
						)
					}
					thread.current = truthOutcome.frame
					continue
				}
				if truthOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						truthOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if truthOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid truth special method outcome",
					)
				}
				continue
			}
			if active.length != nil {
				call := active.length
				active.length = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"length special method has no caller",
					)
				}
				lengthOutcome, lengthErr := finishLengthCall(thread.current, call, result)
				if lengthErr != nil {
					return nil, nil, lengthErr
				}
				if lengthOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						lengthOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if lengthOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid length special method outcome",
					)
				}
				continue
			}
			if active.hash != nil {
				call := active.hash
				active.hash = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"hash special method has no caller",
					)
				}
				hashOutcome, hashErr := finishHashCall(thread.current, call, result)
				if hashErr != nil {
					return nil, nil, hashErr
				}
				if hashOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						hashOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if hashOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid hash special method outcome",
					)
				}
				continue
			}
			if active.representation != nil {
				call := active.representation
				active.representation = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"representation method has no caller",
					)
				}
				representationOutcome, representationErr := finishRepresentationCall(
					thread.current,
					call,
					result,
				)
				if representationErr != nil {
					return nil, nil, representationErr
				}
				if representationOutcome.kind == called {
					if representationOutcome.frame == nil ||
						representationOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid chained representation frame transition",
						)
					}
					thread.current = representationOutcome.frame
					continue
				}
				if representationOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						representationOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if representationOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid representation method outcome",
					)
				}
				continue
			}
			if active.iteration != nil {
				call := active.iteration
				active.iteration = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"iteration special method has no caller",
					)
				}
				iterationOutcome, iterationErr := finishIterationCall(
					thread.current,
					call,
					result,
				)
				if iterationErr != nil {
					return nil, nil, iterationErr
				}
				if iterationOutcome.kind == called {
					if iterationOutcome.frame == nil ||
						iterationOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid chained iteration frame transition",
						)
					}
					thread.current = iterationOutcome.frame
					continue
				}
				if iterationOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						iterationOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if iterationOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid iteration special method outcome",
					)
				}
				continue
			}
			if active.membership != nil {
				call := active.membership
				active.membership = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"containment special method has no caller",
					)
				}
				membershipOutcome, membershipErr := finishMembershipCall(
					thread.current,
					call,
					result,
				)
				if membershipErr != nil {
					return nil, nil, membershipErr
				}
				switch membershipOutcome.kind {
				case advance:
					continue
				case called:
					if membershipOutcome.frame == nil ||
						membershipOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid containment truth call transition",
						)
					}
					thread.current = membershipOutcome.frame
					continue
				case raised:
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						membershipOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				default:
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid containment special method outcome",
					)
				}
			}
			if active.subscription != nil {
				call := active.subscription
				active.subscription = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"subscription special method has no caller",
					)
				}
				subscriptionOutcome, subscriptionErr := finishSubscriptionCall(
					thread.current,
					call,
					result,
				)
				if subscriptionErr != nil {
					return nil, nil, subscriptionErr
				}
				if subscriptionOutcome.kind == raised {
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						subscriptionOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				}
				if subscriptionOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid subscription special method outcome",
					)
				}
				continue
			}
			if active.comparison != nil {
				call := active.comparison
				active.comparison = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"comparison special method has no caller",
					)
				}
				comparisonOutcome, comparisonErr := finishComparisonCall(
					thread.current,
					call,
					result,
				)
				if comparisonErr != nil {
					return nil, nil, comparisonErr
				}
				switch comparisonOutcome.kind {
				case advance:
					continue
				case called:
					if comparisonOutcome.frame == nil ||
						comparisonOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid comparison call transition",
						)
					}
					thread.current = comparisonOutcome.frame
					continue
				case raised:
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						comparisonOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				default:
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid comparison special method outcome",
					)
				}
			}
			if active.unary != nil {
				call := active.unary
				active.unary = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"unary special method has no caller",
					)
				}
				unaryOutcome, unaryErr := finishUnaryCall(thread.current, call, result)
				if unaryErr != nil {
					return nil, nil, unaryErr
				}
				if unaryOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid unary special method outcome",
					)
				}
				continue
			}
			if active.binary != nil {
				call := active.binary
				active.binary = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"binary special method has no caller",
					)
				}
				binaryOutcome, binaryErr := finishBinaryCall(thread.current, call, result)
				if binaryErr != nil {
					return nil, nil, binaryErr
				}
				switch binaryOutcome.kind {
				case advance:
					continue
				case called:
					if binaryOutcome.frame == nil ||
						binaryOutcome.frame.previous != thread.current {
						return nil, nil, thread.current.failure(
							call.instruction,
							"invalid binary call transition",
						)
					}
					thread.current = binaryOutcome.frame
					continue
				case raised:
					unhandled, routeErr := routeException(
						thread,
						thread.current,
						call.instruction,
						binaryOutcome.exception,
						false,
					)
					if routeErr != nil {
						return nil, nil, routeErr
					}
					if unhandled != nil {
						return nil, unhandled, nil
					}
					continue
				default:
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid binary special method outcome",
					)
				}
			}
			if active.attribute != nil {
				call := active.attribute
				active.attribute = nil
				if thread.current == nil {
					return nil, nil, active.failure(
						index,
						"descriptor special method has no caller",
					)
				}
				attributeBuiltin := active.attributeBuiltin
				active.attributeBuiltin = nil
				if attributeBuiltin != nil && attributeBuiltin.presence {
					if !thread.current.push(trueSingleton) {
						return nil, nil, thread.current.failure(
							attributeBuiltin.instruction,
							"operand stack overflow while returning hasattr result",
						)
					}
					continue
				}
				attributeOutcome, attributeErr := finishAttributeCall(
					thread.current,
					call,
					result,
				)
				if attributeErr != nil {
					return nil, nil, attributeErr
				}
				if attributeOutcome.kind != advance {
					return nil, nil, thread.current.failure(
						call.instruction,
						"invalid descriptor special method outcome",
					)
				}
				continue
			}
			if active.attributeBuiltin != nil {
				if active.attributeBuiltin.presence {
					result = trueSingleton
				}
				active.attributeBuiltin = nil
			}
			if active.moduleImport != nil {
				loaded := active.moduleImport
				active.moduleImport = nil
				if thread.current == nil || thread.current.instruction == 0 {
					return nil, nil, active.failure(index, "import frame has no suspended caller")
				}
				if thread.current.pendingImport != nil {
					return nil, nil, active.failure(index, "caller already has a pending import")
				}
				thread.current.pendingImport = loaded.request
				thread.current.instruction--
				continue
			}
			if thread.current == nil {
				return result, nil, nil
			}
			if !thread.current.push(result) {
				return nil, nil, thread.current.failure(
					thread.current.instruction,
					"operand stack overflow while returning to caller",
				)
			}
		case raised:
			unhandled, routeErr := routeException(
				thread,
				active,
				index,
				outcome.exception,
				outcome.reraise,
			)
			if routeErr != nil {
				return nil, nil, routeErr
			}
			if unhandled != nil {
				return nil, unhandled, nil
			}
		default:
			return nil, nil, active.failure(index, "unknown execution outcome")
		}
	}
	return nil, nil, &BytecodeError{Instruction: -1, Message: "execution has no frame"}
}

// routeException searches the current frame and its callers for a protected
// instruction range, restoring the selected handler's stack before resuming.
func routeException(
	thread *threadState,
	origin *frame,
	instruction int,
	exception *Exception,
	reraise bool,
) (*raisedOutcome, error) {
	if origin == nil {
		return nil, &BytecodeError{Instruction: instruction, Message: "exception has no frame"}
	}
	if exception == nil {
		return nil, origin.failure(instruction, "raised outcome has no exception")
	}
	current := origin
	currentInstruction := instruction
route:
	for {
		for {
			delegate, delegatedAt, delegatedException, forwarded, forwardErr :=
				forwardDelegatedException(current, currentInstruction, exception)
			if forwardErr != nil {
				return nil, forwardErr
			}
			if !forwarded {
				break
			}
			current = delegate
			currentInstruction = delegatedAt
			exception = delegatedException
			thread.current = current
		}
		if exception.originFrame == nil {
			exception.chainContext(activeHandledException(current, currentInstruction))
			exception.originFrame = current
			exception.originInstruction = currentInstruction
		}
		unhandled := &raisedOutcome{
			exception:   exception,
			frame:       exception.originFrame,
			instruction: exception.originInstruction,
		}
		skipTraceback := reraise
		for current != nil {
			if skipTraceback {
				skipTraceback = false
			} else {
				exception.traceback = append(exception.traceback, tracebackEntry{
					frame:       current,
					instruction: currentInstruction,
				})
			}
			if handler, ok := current.code.exceptionHandler(currentInstruction); ok {
				if current.delegation != nil &&
					(currentInstruction == current.delegation.sendInstruction ||
						currentInstruction == current.delegation.yieldInstruction) {
					current.delegation = nil
				}
				depth := handler.StackDepth
				if len(current.stack) < depth {
					return nil, current.failure(
						currentInstruction,
						"exception handler stack depth exceeds operand stack",
					)
				}
				for index := depth; index < len(current.stack); index++ {
					current.stack[index] = nil
				}
				current.stack = current.stack[:depth]
				if !current.push(exception) {
					return nil, current.failure(
						currentInstruction,
						"operand stack overflow while entering exception handler",
					)
				}
				current.instruction = int(handler.Target)
				thread.current = current
				return nil, nil
			}

			if current.attributeBuiltin != nil && isAttributeError(exception) {
				call := current.attributeBuiltin
				current.attributeBuiltin = nil
				for index := range current.stack {
					current.stack[index] = nil
				}
				current.stack = current.stack[:0]
				current.discardImportedModule()
				caller := current.previous
				if caller == nil {
					return nil, current.failure(
						currentInstruction,
						"getattr attribute call has no caller",
					)
				}
				if !caller.push(call.attributeError) {
					return nil, caller.failure(
						call.instruction,
						"operand stack overflow while returning attribute fallback",
					)
				}
				thread.current = caller
				return nil, nil
			}

			if current.iteration != nil &&
				current.iteration.kind != iterationGetIterator &&
				current.iteration.kind != iterationCollectionIterator &&
				current.iteration.kind != iterationEnumerateIterator &&
				current.iteration.kind != iterationTruthAggregateIterator &&
				current.iteration.kind != iterationMapIterator &&
				current.iteration.kind != iterationFilterIterator &&
				current.iteration.kind != iterationZipIterator &&
				isStopIteration(exception) {
				call := current.iteration
				current.iteration = nil
				for index := range current.stack {
					current.stack[index] = nil
				}
				current.stack = current.stack[:0]
				current.discardImportedModule()
				caller := current.previous
				if caller == nil {
					return nil, current.failure(
						currentInstruction,
						"iteration special method has no caller",
					)
				}
				iterationOutcome, iterationErr := finishIterationStop(
					caller,
					call,
					exception,
				)
				if iterationErr != nil {
					return nil, iterationErr
				}
				thread.current = caller
				if iterationOutcome.kind == advance {
					return nil, nil
				}
				if iterationOutcome.kind == called {
					if iterationOutcome.frame == nil ||
						iterationOutcome.frame.previous != caller {
						return nil, caller.failure(
							call.instruction,
							"invalid iteration exhaustion frame transition",
						)
					}
					thread.current = iterationOutcome.frame
					return nil, nil
				}
				if iterationOutcome.kind != raised {
					return nil, caller.failure(
						call.instruction,
						"invalid iteration exhaustion outcome",
					)
				}
				exception = iterationOutcome.exception
				current = caller
				currentInstruction = call.instruction
				continue route
			}

			for index := range current.stack {
				current.stack[index] = nil
			}
			current.discardImportedModule()
			caller := current.previous
			if current.generator != nil {
				generator := current.generator
				if generator.state != generatorRunning {
					return nil, current.failure(
						currentInstruction,
						"exception left a generator that is not running",
					)
				}
				resumeKind := generator.resume.kind
				switch resumeKind {
				case generatorAsyncNext:
					if generator.resume.asyncNext == nil {
						return nil, current.failure(
							currentInstruction,
							"async generator failure has no next awaitable",
						)
					}
					generator.resume.asyncNext.state = asyncGeneratorNextClosed
				case generatorAsyncThrow:
					if generator.resume.asyncThrow == nil {
						return nil, current.failure(
							currentInstruction,
							"async generator failure has no athrow awaitable",
						)
					}
					generator.resume.asyncThrow.state = asyncGeneratorNextClosed
				}
				if resumeKind == generatorAsyncThrow &&
					generator.resume.asyncThrow.close &&
					(exception.class != nil &&
						exception.class.isSubclassOf(generatorExitType) ||
						isStopAsyncIteration(exception)) {
					resume := generator.resume
					if err := completeAsyncGeneratorClose(
						current,
						caller,
						resume.instruction,
						resume.target,
						resume.asyncThrow,
					); err != nil {
						return nil, err
					}
					generator.complete()
					thread.current = caller
					return nil, nil
				}
				if resumeKind == generatorClose && exception.class != nil &&
					exception.class.isSubclassOf(generatorExitType) {
					generator.complete()
					if caller == nil {
						return nil, current.failure(
							currentInstruction,
							"generator close has no caller",
						)
					}
					if !caller.push(None) {
						return nil, caller.failure(
							caller.instruction-1,
							"operand stack overflow while completing generator close",
						)
					}
					thread.current = caller
					return nil, nil
				}
				if isStopIteration(exception) ||
					(generator.kind == asyncGeneratorObject &&
						isStopAsyncIteration(exception)) {
					exception = transformGeneratorStopIteration(
						exception,
						current,
						currentInstruction,
					)
					unhandled = &raisedOutcome{
						exception:   exception,
						frame:       current,
						instruction: currentInstruction,
					}
				}
				if resumeKind == generatorDelegateClose {
					pending, closeErr := takeDelegatedClose(
						caller,
						generator,
						currentInstruction,
					)
					if closeErr != nil {
						return nil, closeErr
					}
					generator.complete()
					if exception.class != nil &&
						exception.class.isSubclassOf(generatorExitType) {
						exception = pending
						current = caller
						currentInstruction = current.instruction - 1
						if currentInstruction < 0 {
							return nil, current.failure(
								currentInstruction,
								"delegated close caller has no active instruction",
							)
						}
						reraise = false
						thread.current = current
						continue route
					}
				} else {
					generator.complete()
				}
			}
			if caller == nil {
				thread.current = nil
				return unhandled, nil
			}
			current = caller
			currentInstruction = current.instruction - 1
			if currentInstruction < 0 {
				return nil, current.failure(
					currentInstruction,
					"caller has no active call instruction",
				)
			}
			thread.current = current
		}
		return unhandled, nil
	}
}

// executeInstruction applies one validated operation and reports whether the
// frame advances, returns a value, or raises a Python exception.
func executeInstruction(
	frame *frame,
	index int,
	instruction bytecode.Instruction,
) (instructionOutcome, error) {
	switch instruction.Opcode {
	case bytecode.Nop:
		return instructionOutcome{kind: advance}, nil
	case bytecode.LoadConst:
		value := frame.code.constants[instruction.Operand]
		return pushOutcome(frame, index, value)
	case bytecode.ConvertValue:
		return executeConvertValue(frame, index, instruction.Operand)
	case bytecode.FormatSimple:
		return executeFormatSimple(frame, index)
	case bytecode.FormatWithSpec:
		return executeFormatWithSpec(frame, index)
	case bytecode.BuildString:
		return executeBuildString(frame, index, int(instruction.Operand))
	case bytecode.BuildInterpolation:
		return executeBuildInterpolation(frame, index, instruction.Operand)
	case bytecode.BuildTemplate:
		return executeBuildTemplate(frame, index)
	case bytecode.LoadNotImplementedError:
		return pushOutcome(frame, index, newException("NotImplementedError", ""))
	case bytecode.LoadAssertionError:
		return pushOutcome(frame, index, assertionErrorType)
	case bytecode.LoadStopAsyncIteration:
		return pushOutcome(frame, index, stopAsyncIterationType)
	case bytecode.LoadBuildClass:
		return pushOutcome(frame, index, buildClassSingleton)
	case bytecode.LoadName:
		name := frame.code.names[instruction.Operand]
		value, ok := frame.lookupName(name)
		if !ok {
			return instructionOutcome{
				kind:      raised,
				exception: newException("NameError", fmt.Sprintf("name '%s' is not defined", name)),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadFast:
		localIndex := int(instruction.Operand)
		value := frame.fastLocals[localIndex]
		if value == nil {
			name := frame.code.locals[localIndex]
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"UnboundLocalError",
					"cannot access local variable '"+name+
						"' where it is not associated with a value",
				),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadDeref:
		derefIndex := int(instruction.Operand)
		value := frame.deref[derefIndex].value
		if value == nil {
			return instructionOutcome{
				kind:      raised,
				exception: unboundDerefException(frame.code, derefIndex),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadClosure:
		return pushOutcome(frame, index, frame.deref[instruction.Operand])
	case bytecode.LoadHandledExceptionType:
		return executeLoadHandledExceptionType(frame, index)
	case bytecode.LoadGlobal:
		name := frame.code.names[instruction.Operand]
		value, ok := frame.globals.get(name)
		if !ok {
			value, ok = frame.builtins.get(name)
		}
		if !ok {
			return instructionOutcome{
				kind:      raised,
				exception: newException("NameError", fmt.Sprintf("name '%s' is not defined", name)),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadLocals:
		return pushOutcome(frame, index, &namespaceValue{namespace: frame.locals})
	case bytecode.LoadFromDictOrGlobals:
		return executeLoadFromDictOrGlobals(
			frame,
			index,
			frame.code.names[instruction.Operand],
		)
	case bytecode.LoadFromDictOrDeref:
		return executeLoadFromDictOrDeref(frame, index, int(instruction.Operand))
	case bytecode.LoadSpecial:
		return executeLoadSpecial(frame, index, frame.code.names[instruction.Operand])
	case bytecode.LoadAttr:
		owner, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		name := frame.code.names[instruction.Operand]
		switch owner := owner.(type) {
		case *typeVarValue:
			switch name {
			case "__name__":
				return pushOutcome(frame, index, &stringValue{value: owner.name})
			case "__bound__":
				return executeTypeVarLoad(frame, index, owner, typeVarBoundLoad)
			case "__constraints__":
				return executeTypeVarLoad(frame, index, owner, typeVarConstraintsLoad)
			case "__default__":
				return executeTypeVarLoad(frame, index, owner, typeVarDefaultLoad)
			case "__covariant__", "__contravariant__":
				return pushOutcome(frame, index, falseSingleton)
			case "__infer_variance__":
				return pushOutcome(frame, index, trueSingleton)
			default:
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"'typing.TypeVar' object has no attribute '"+name+"'",
					),
				}, nil
			}
		case *typeVarTupleValue:
			switch name {
			case "__name__":
				return pushOutcome(frame, index, &stringValue{value: owner.name})
			case "__default__":
				return executeVariadicTypeParameterDefaultLoad(frame, index, owner)
			default:
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"'typing.TypeVarTuple' object has no attribute '"+name+"'",
					),
				}, nil
			}
		case *paramSpecValue:
			switch name {
			case "__name__":
				return pushOutcome(frame, index, &stringValue{value: owner.name})
			case "__bound__":
				return pushOutcome(frame, index, None)
			case "__covariant__", "__contravariant__":
				return pushOutcome(frame, index, falseSingleton)
			case "__infer_variance__":
				return pushOutcome(frame, index, trueSingleton)
			case "__default__":
				return executeVariadicTypeParameterDefaultLoad(frame, index, owner)
			case "args":
				return pushOutcome(frame, index, &paramSpecArgsValue{parameter: owner})
			case "kwargs":
				return pushOutcome(frame, index, &paramSpecKwargsValue{parameter: owner})
			default:
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"'typing.ParamSpec' object has no attribute '"+name+"'",
					),
				}, nil
			}
		case *typeAliasValue:
			switch name {
			case "__name__":
				return pushOutcome(frame, index, &stringValue{value: owner.name})
			case "__module__":
				return pushOutcome(frame, index, owner.module)
			case "__type_params__":
				return pushOutcome(frame, index, owner.typeParams)
			case "__value__":
				return executeTypeAliasValueLoad(frame, index, owner)
			default:
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"'typing.TypeAliasType' object has no attribute '"+name+"'",
					),
				}, nil
			}
		case *functionValue:
			return executeFunctionAttributeLoad(frame, index, owner, name)
		case *classMethodValue:
			return executeMethodDescriptorAttributeLoad(
				frame,
				index,
				owner,
				owner.callable,
				name,
			)
		case *staticMethodValue:
			return executeMethodDescriptorAttributeLoad(
				frame,
				index,
				owner,
				owner.callable,
				name,
			)
		case *propertyValue:
			return executePropertyAttributeLoad(frame, index, owner, name)
		case *templateValue:
			return executeTemplateAttributeLoad(frame, index, owner, name)
		case *interpolationValue:
			return executeInterpolationAttributeLoad(frame, index, owner, name)
		case *cmpKeyValue:
			return executeCmpKeyAttributeLoad(frame, index, owner, name)
		case *stringValue:
			return executeStringAttributeLoad(frame, index, owner, name)
		case *dictValue:
			return executeDictionaryAttributeLoad(frame, index, owner, name)
		case *listValue:
			return executeListAttributeLoad(frame, index, owner, name)
		case *setValue:
			return executeSetAttributeLoad(frame, index, owner, name)
		case *frozenSetValue:
			return executeFrozenSetAttributeLoad(frame, index, owner, name)
		case *rangeValue:
			return executeRangeAttributeLoad(frame, index, owner, name)
		case *superValue:
			return executeSuperAttributeLoad(frame, index, owner, name)
		case *generatorValue:
			if owner.kind == asyncGeneratorObject {
				switch name {
				case "__aiter__":
					return pushOutcome(
						frame,
						index,
						&asyncGeneratorAIterMethod{generator: owner},
					)
				case "__anext__":
					return pushOutcome(
						frame,
						index,
						&asyncGeneratorANextMethod{generator: owner},
					)
				case "asend":
					return pushOutcome(
						frame,
						index,
						&asyncGeneratorASendMethod{generator: owner},
					)
				case "athrow":
					return pushOutcome(
						frame,
						index,
						&asyncGeneratorAThrowMethod{generator: owner},
					)
				case "aclose":
					return pushOutcome(
						frame,
						index,
						&asyncGeneratorACloseMethod{generator: owner},
					)
				default:
					return instructionOutcome{
						kind: raised,
						exception: newException(
							"AttributeError",
							"'async_generator' object has no attribute '"+name+"'",
						),
					}, nil
				}
			}
			switch name {
			case "send":
				return pushOutcome(frame, index, &generatorSendMethod{generator: owner})
			case "throw":
				return pushOutcome(frame, index, &generatorThrowMethod{generator: owner})
			case "close":
				return pushOutcome(frame, index, &generatorCloseMethod{generator: owner})
			default:
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
					),
				}, nil
			}
		case *Exception:
			return executeExceptionAttributeLoad(frame, index, owner, name)
		case *Module:
			return executeModuleAttributeLoad(frame, index, owner, name)
		case *nativeTypeValue:
			return executeNativeTypeAttributeLoad(frame, index, owner, name)
		case *exceptionTypeValue:
			return executeExceptionTypeAttributeLoad(frame, index, owner, name)
		case *typeValue:
			return executeClassAttributeLoad(frame, index, owner, name)
		case *instanceValue:
			return executeInstanceAttributeLoad(frame, index, owner, name)
		default:
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"AttributeError",
					"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
				),
			}, nil
		}
	case bytecode.StoreAttr:
		owner, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		name := frame.code.names[instruction.Operand]
		return executeDynamicAttributeStore(frame, index, owner, name, value)
	case bytecode.DeleteAttr:
		owner, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		name := frame.code.names[instruction.Operand]
		return executeDynamicAttributeDelete(frame, index, owner, name)
	case bytecode.StoreName:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		name := frame.code.names[instruction.Operand]
		frame.locals.values[name] = value
		if frame.classBuild != nil {
			frame.classBuild.recordStore(name)
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.DeleteName:
		name := frame.code.names[instruction.Operand]
		if _, found := frame.locals.values[name]; !found {
			return instructionOutcome{
				kind:      raised,
				exception: newException("NameError", "name '"+name+"' is not defined"),
			}, nil
		}
		delete(frame.locals.values, name)
		if frame.classBuild != nil {
			delete(frame.classBuild.namespacePosition, name)
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.StoreFast:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.fastLocals[instruction.Operand] = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.DeleteFast:
		localIndex := int(instruction.Operand)
		if frame.fastLocals[localIndex] == nil {
			name := frame.code.locals[localIndex]
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"UnboundLocalError",
					"cannot access local variable '"+name+
						"' where it is not associated with a value",
				),
			}, nil
		}
		frame.fastLocals[localIndex] = nil
		return instructionOutcome{kind: advance}, nil
	case bytecode.StoreDeref:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.deref[instruction.Operand].value = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.DeleteDeref:
		derefIndex := int(instruction.Operand)
		cell := frame.deref[derefIndex]
		if cell.value == nil {
			return instructionOutcome{
				kind:      raised,
				exception: unboundDerefException(frame.code, derefIndex),
			}, nil
		}
		cell.value = nil
		return instructionOutcome{kind: advance}, nil
	case bytecode.StoreGlobal:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		frame.globals.values[frame.code.names[instruction.Operand]] = value
		return instructionOutcome{kind: advance}, nil
	case bytecode.DeleteGlobal:
		name := frame.code.names[instruction.Operand]
		if _, found := frame.globals.values[name]; !found {
			return instructionOutcome{
				kind:      raised,
				exception: newException("NameError", "name '"+name+"' is not defined"),
			}, nil
		}
		delete(frame.globals.values, name)
		return instructionOutcome{kind: advance}, nil
	case bytecode.Copy:
		depth := int(instruction.Operand)
		if depth < 1 || depth > len(frame.stack) {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return pushOutcome(frame, index, frame.stack[len(frame.stack)-depth])
	case bytecode.Swap:
		depth := int(instruction.Operand)
		if depth < 2 || depth > len(frame.stack) {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		top := len(frame.stack) - 1
		other := len(frame.stack) - depth
		frame.stack[top], frame.stack[other] = frame.stack[other], frame.stack[top]
		return instructionOutcome{kind: advance}, nil
	case bytecode.PopTop:
		if _, ok := frame.pop(); !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.Jump:
		frame.instruction = int(instruction.Operand)
		return instructionOutcome{kind: advance}, nil
	case bytecode.PopJumpIfFalse, bytecode.PopJumpIfTrue:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return executeTruthOperation(frame, index, instruction, value)
	case bytecode.JumpIfFalseOrPop, bytecode.JumpIfTrueOrPop:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return executeTruthOperation(frame, index, instruction, value)
	case bytecode.BuildTuple:
		return executeBuildSequence(frame, index, int(instruction.Operand), true)
	case bytecode.BuildList:
		return executeBuildSequence(frame, index, int(instruction.Operand), false)
	case bytecode.BuildSet:
		return executeBuildSet(frame, index, int(instruction.Operand))
	case bytecode.BuildMap:
		return executeBuildMap(frame, index, int(instruction.Operand))
	case bytecode.MapSet:
		return executeMapSet(frame, index)
	case bytecode.MapUpdate:
		return executeMapUpdate(frame, index)
	case bytecode.MapMerge:
		return executeMapMerge(frame, index)
	case bytecode.ImportName:
		return executeImportName(frame, index, frame.code.names[instruction.Operand])
	case bytecode.ImportFrom:
		return executeImportFrom(frame, index, frame.code.names[instruction.Operand])
	case bytecode.ImportStar:
		return executeImportStar(frame, index)
	case bytecode.MakeFunction:
		function := &functionValue{
			code:       frame.code.children[instruction.Operand],
			globals:    frame.globals,
			typeParams: &tupleValue{},
		}
		return pushOutcome(frame, index, function)
	case bytecode.MakeTypeAlias:
		return executeMakeTypeAlias(frame, index)
	case bytecode.MakeTypeVar, bytecode.MakeTypeVarTuple, bytecode.MakeParamSpec:
		return executeMakeTypeParameter(frame, index, instruction.Opcode)
	case bytecode.SetTypeAliasParameters:
		return executeSetTypeAliasParameters(frame, index)
	case bytecode.SetFunctionTypeParameters:
		return executeSetFunctionTypeParameters(frame, index)
	case bytecode.SetTypeVarBound:
		return executeSetTypeVarEvaluator(frame, index, typeVarBoundLoad)
	case bytecode.SetTypeVarConstraints:
		return executeSetTypeVarEvaluator(frame, index, typeVarConstraintsLoad)
	case bytecode.SetTypeVarDefault:
		return executeSetTypeVarEvaluator(frame, index, typeVarDefaultLoad)
	case bytecode.SetFunctionAttribute:
		target, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		payload, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		function, ok := target.(*functionValue)
		if !ok {
			return instructionOutcome{}, frame.failure(
				index,
				"SET_FUNCTION_ATTRIBUTE target is not a function",
			)
		}
		switch bytecode.FunctionAttribute(instruction.Operand) {
		case bytecode.FunctionDefaults:
			defaults, defaultsOK := payload.(*tupleValue)
			if !defaultsOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function defaults payload is not a tuple",
				)
			}
			if len(defaults.elements) > function.code.code.PositionalCount() {
				return instructionOutcome{}, frame.failure(
					index,
					"function default count exceeds positional parameter count",
				)
			}
			function.defaults = make([]Value, len(defaults.elements))
			copy(function.defaults, defaults.elements)
		case bytecode.FunctionKeywordDefaults:
			defaults, defaultsOK := payload.(*dictValue)
			if !defaultsOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function keyword defaults payload is not a dictionary",
				)
			}
			function.keywordDefaults = make(map[string]Value, len(defaults.entries))
			start, end := keywordOnlyRange(function.code)
			for _, entry := range defaults.entries {
				name, nameOK := entry.key.(*stringValue)
				if !nameOK {
					return instructionOutcome{}, frame.failure(
						index,
						"function keyword default name is not a string",
					)
				}
				found := false
				for parameter := start; parameter < end; parameter++ {
					if function.code.locals[parameter] == name.value {
						found = true
						break
					}
				}
				if !found {
					return instructionOutcome{}, frame.failure(
						index,
						"function keyword default has no keyword-only parameter",
					)
				}
				function.keywordDefaults[name.value] = entry.value
			}
		case bytecode.FunctionClosure:
			closure, closureOK := payload.(*tupleValue)
			if !closureOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function closure payload is not a tuple",
				)
			}
			if len(closure.elements) != len(function.code.freeVars) {
				return instructionOutcome{}, frame.failure(
					index,
					fmt.Sprintf(
						"function closure has %d cells for %d free variables",
						len(closure.elements),
						len(function.code.freeVars),
					),
				)
			}
			function.closure = make([]*cellValue, len(closure.elements))
			for closureIndex, value := range closure.elements {
				cell, cellOK := value.(*cellValue)
				if !cellOK {
					return instructionOutcome{}, frame.failure(
						index,
						fmt.Sprintf(
							"function closure item %d is not a cell",
							closureIndex,
						),
					)
				}
				function.closure[closureIndex] = cell
			}
		case bytecode.FunctionAnnotate:
			annotation, annotationOK := payload.(*functionValue)
			if !annotationOK {
				return instructionOutcome{}, frame.failure(
					index,
					"function annotate payload is not a function",
				)
			}
			function.annotate = annotation
		}
		return pushOutcome(frame, index, function)
	case bytecode.Call:
		return executeCall(frame, index, int(instruction.Operand))
	case bytecode.CallEx:
		return executeUnpackedCall(
			frame,
			index,
			instruction.Operand == bytecode.CallExWithKeywords,
		)
	case bytecode.SetAdd:
		return executeSetAdd(frame, index)
	case bytecode.SetUpdate:
		return executeSetUpdate(frame, index)
	case bytecode.ListAppend:
		return executeListAppend(frame, index)
	case bytecode.ListExtend:
		return executeListExtend(frame, index)
	case bytecode.ListToTuple:
		return executeListToTuple(frame, index)
	case bytecode.BuildSlice:
		return executeBuildSlice(frame, index, int(instruction.Operand))
	case bytecode.GetIter:
		iterable, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return executeIteratorLookup(frame, index, iterable)
	case bytecode.GetAwaitable:
		return executeGetAwaitable(frame, index, instruction.Operand)
	case bytecode.CheckAsyncIterator:
		return executeCheckAsyncIterator(frame, index)
	case bytecode.MatchSequence:
		return executeMatchSequence(frame, index)
	case bytecode.GetLen:
		return executeGetLen(frame, index)
	case bytecode.MatchMapping:
		return executeMatchMapping(frame, index)
	case bytecode.MatchMappingKey:
		return executeMatchMappingKey(frame, index)
	case bytecode.CopyMapping:
		return executeCopyMapping(frame, index)
	case bytecode.CheckMappingKey:
		return executeCheckMappingKey(frame, index)
	case bytecode.MatchClass:
		return executeMatchClass(frame, index, int(instruction.Operand))
	case bytecode.ForIter:
		return executeForIter(frame, index, int(instruction.Operand))
	case bytecode.Send:
		return executeSend(frame, index, int(instruction.Operand))
	case bytecode.BinarySubscript:
		return executeBinarySubscript(frame, index)
	case bytecode.StoreSubscript:
		return executeStoreSubscript(frame, index)
	case bytecode.DeleteSubscript:
		return executeDeleteSubscript(frame, index)
	case bytecode.UnpackSequence:
		return executeUnpackSequence(frame, index, int(instruction.Operand))
	case bytecode.UnpackEx:
		before, after := bytecode.UnpackExCounts(instruction.Operand)
		return executeUnpackEx(frame, index, int(before), int(after))
	case bytecode.UnaryOp:
		return executeUnary(frame, index, instruction.Operand)
	case bytecode.BinaryOp:
		return executeBinary(frame, index, instruction.Operand, false)
	case bytecode.InplaceOp:
		return executeBinary(frame, index, instruction.Operand, true)
	case bytecode.CompareOp:
		return executeComparison(frame, index, instruction.Operand)
	case bytecode.CheckExceptionMatch:
		handlerType, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		if len(frame.stack) == 0 {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		exception, ok := frame.stack[len(frame.stack)-1].(*Exception)
		if !ok {
			return instructionOutcome{}, frame.failure(
				index,
				"CHECK_EXC_MATCH left operand is not an exception",
			)
		}
		matches, matchError := matchException(exception, handlerType)
		if matchError != nil {
			return instructionOutcome{kind: raised, exception: matchError}, nil
		}
		result := falseSingleton
		if matches {
			result = trueSingleton
		}
		return pushOutcome(frame, index, result)
	case bytecode.CheckExceptionGroupMatch:
		return executeExceptionGroupMatch(frame, index)
	case bytecode.PrepareReraiseStar:
		return executePrepareReraiseStar(frame, index)
	case bytecode.EnterExcept:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		exception, ok := value.(*Exception)
		if !ok {
			return instructionOutcome{}, frame.failure(
				index,
				"ENTER_EXCEPT value is not an exception",
			)
		}
		frame.handledExceptions = append(frame.handledExceptions, handledException{
			exception: exception,
			start:     frame.instruction,
			end:       int(instruction.Operand),
		})
		return instructionOutcome{kind: advance}, nil
	case bytecode.LeaveExcept:
		if len(frame.handledExceptions) == 0 {
			return instructionOutcome{}, frame.failure(index, "LEAVE_EXCEPT has no active handler")
		}
		last := len(frame.handledExceptions) - 1
		frame.handledExceptions[last].exception = nil
		frame.handledExceptions = frame.handledExceptions[:last]
		return instructionOutcome{kind: advance}, nil
	case bytecode.Reraise:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		exception, ok := value.(*Exception)
		if !ok {
			return instructionOutcome{}, frame.failure(index, "RERAISE value is not an exception")
		}
		return instructionOutcome{
			kind:      raised,
			exception: exception,
			reraise:   exception.originFrame != nil,
		}, nil
	case bytecode.RaiseVarargs:
		if instruction.Operand == 0 {
			exception := activeHandledException(frame, index)
			reraise := true
			if exception == nil {
				exception = newException("RuntimeError", "No active exception to reraise")
				reraise = false
			}
			return instructionOutcome{kind: raised, exception: exception, reraise: reraise}, nil
		}
		var causeValue Value
		if instruction.Operand == 2 {
			var ok bool
			causeValue, ok = frame.pop()
			if !ok {
				return instructionOutcome{}, frame.failure(index, "operand stack underflow")
			}
		}
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		exception, normalizationFailure := normalizeRaisedValue(
			value,
			"exceptions must derive from BaseException",
		)
		if normalizationFailure != nil {
			return instructionOutcome{kind: raised, exception: normalizationFailure}, nil
		}
		if instruction.Operand == 2 {
			var cause *Exception
			if causeValue != None {
				var causeFailure *Exception
				cause, causeFailure = normalizeRaisedValue(
					causeValue,
					"exception causes must derive from BaseException",
				)
				if causeFailure != nil {
					return instructionOutcome{kind: raised, exception: causeFailure}, nil
				}
			}
			exception.cause = cause
			exception.suppressContext = true
		}
		exception.originFrame = nil
		exception.originInstruction = 0
		return instructionOutcome{kind: raised, exception: exception}, nil
	case bytecode.AsyncGenWrap:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return pushOutcome(frame, index, &asyncGeneratorWrappedValue{value: value})
	case bytecode.YieldValue:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return instructionOutcome{kind: yielded, value: value}, nil
	case bytecode.ReturnValue:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return instructionOutcome{kind: returned, value: value}, nil
	default:
		return instructionOutcome{}, frame.failure(
			index,
			"unsupported opcode reached dispatch: "+instruction.Opcode.String(),
		)
	}
}

func pushOutcome(frame *frame, index int, value Value) (instructionOutcome, error) {
	if !frame.push(value) {
		return instructionOutcome{}, frame.failure(index, "operand stack overflow")
	}
	return instructionOutcome{kind: advance}, nil
}
