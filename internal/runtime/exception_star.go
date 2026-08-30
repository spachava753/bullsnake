package runtime

import (
	"fmt"
	"slices"
)

const cannotCatchGroupWithStarMessage = "catching ExceptionGroup with except* is not allowed. Use except instead."

// executeExceptionGroupMatch validates the handler class, consumes the current
// remainder, and pushes its recursively split rest and match values.
func executeExceptionGroupMatch(frame *frame, instruction int) (instructionOutcome, error) {
	handlerType, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	candidate, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	if matchError := validateExceptionStarHandlerType(handlerType); matchError != nil {
		return raiseOutcome(matchError), nil
	}
	if candidate == None {
		return pushExceptionGroupMatch(frame, instruction, nil, nil)
	}
	exception, ok := candidate.(*Exception)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"CHECK_EG_MATCH left operand is not an exception or None",
		)
	}
	match, rest := splitExceptionForStarLevel(exception, handlerType, true)
	return pushExceptionGroupMatch(frame, instruction, rest, match)
}

func validateExceptionStarHandlerType(handlerType Value) *Exception {
	if !validExceptionHandlerType(handlerType) {
		return newException("TypeError", cannotCatchMessage)
	}
	if exceptionStarHandlerContainsGroup(handlerType) {
		return newException("TypeError", cannotCatchGroupWithStarMessage)
	}
	return nil
}

// exceptionStarHandlerContainsGroup rejects a group class anywhere in the flat
// class tuple accepted by an except* clause.
func exceptionStarHandlerContainsGroup(handlerType Value) bool {
	if tuple, ok := handlerType.(*tupleValue); ok {
		for _, item := range tuple.elements {
			if exceptionStarHandlerContainsGroup(item) {
				return true
			}
		}
		return false
	}
	switch handlerType := handlerType.(type) {
	case *exceptionTypeValue:
		return handlerType.isSubclassOf(baseExceptionGroupType)
	case *typeValue:
		return handlerType.builtinExceptionBase().isSubclassOf(baseExceptionGroupType)
	default:
		return false
	}
}

func pushExceptionGroupMatch(
	frame *frame,
	instruction int,
	rest *Exception,
	match *Exception,
) (instructionOutcome, error) {
	var restValue Value = None
	if rest != nil {
		restValue = rest
	}
	if !frame.push(restValue) {
		return instructionOutcome{}, frame.failure(instruction, "operand stack overflow")
	}
	var matchValue Value = None
	if match != nil {
		matchValue = match
	}
	return pushOutcome(frame, instruction, matchValue)
}

// splitExceptionForStarLevel recursively retains matching and remaining leaves,
// wrapping only a matching naked exception at the initial call.
func splitExceptionForStarLevel(
	exception *Exception,
	handlerType Value,
	wrapNaked bool,
) (match *Exception, rest *Exception) {
	if exceptionMatchesClass(exception, handlerType) {
		if exception.group != nil || !wrapNaked {
			return exception, nil
		}
		children := []Value{exception}
		group := &Exception{
			class:   exceptionGroupTypeForChildren(children),
			message: "",
			group:   &tupleValue{elements: children},
		}
		copyExceptionMetadata(group, exception)
		return group, nil
	}
	if exception.group == nil {
		return nil, exception
	}

	matchedChildren := make([]Value, 0, len(exception.group.elements))
	restChildren := make([]Value, 0, len(exception.group.elements))
	for _, childValue := range exception.group.elements {
		child := childValue.(*Exception)
		childMatch, childRest := splitExceptionForStarLevel(child, handlerType, false)
		if childMatch != nil {
			matchedChildren = append(matchedChildren, childMatch)
		}
		if childRest != nil {
			restChildren = append(restChildren, childRest)
		}
	}
	return deriveExceptionGroup(exception, matchedChildren),
		deriveExceptionGroup(exception, restChildren)
}

func exceptionGroupTypeForChildren(children []Value) *exceptionTypeValue {
	for _, child := range children {
		if !child.(*Exception).class.isSubclassOf(exceptionType) {
			return baseExceptionGroupType
		}
	}
	return exceptionGroupType
}

// deriveExceptionGroup preserves the original group when every child remains;
// otherwise it uses Python's built-in derive class selection and copies metadata.
func deriveExceptionGroup(original *Exception, children []Value) *Exception {
	if len(children) == 0 {
		return nil
	}
	if len(children) == len(original.group.elements) {
		same := true
		for index, child := range children {
			if child != original.group.elements[index] {
				same = false
				break
			}
		}
		if same {
			return original
		}
	}
	derived := &Exception{
		class:   exceptionGroupTypeForChildren(children),
		message: original.message,
		group:   &tupleValue{elements: slices.Clone(children)},
	}
	copyExceptionMetadata(derived, original)
	return derived
}

func copyExceptionMetadata(target, source *Exception) {
	target.cause = source.cause
	target.context = source.context
	target.suppressContext = source.suppressContext
	target.originFrame = source.originFrame
	target.originInstruction = source.originInstruction
	target.traceback = slices.Clone(source.traceback)
}

// executePrepareReraiseStar validates the compiler-owned original and result
// list, then pushes the exception produced by subgroup projection and merging.
func executePrepareReraiseStar(frame *frame, instruction int) (instructionOutcome, error) {
	resultValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	originalValue, ok := frame.pop()
	if !ok {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	original, ok := originalValue.(*Exception)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"PREP_RERAISE_STAR original value is not an exception",
		)
	}
	results, ok := resultValue.(*listValue)
	if !ok {
		return instructionOutcome{}, frame.failure(
			instruction,
			"PREP_RERAISE_STAR result value is not a list",
		)
	}
	for index, result := range results.elements {
		if result == None {
			continue
		}
		if _, ok := result.(*Exception); !ok {
			message := fmt.Sprintf(
				"PREP_RERAISE_STAR result item %d is not an exception or None",
				index,
			)
			return instructionOutcome{}, frame.failure(instruction, message)
		}
	}
	result := prepareReraiseStar(original, results.elements)
	if result == nil {
		return pushOutcome(frame, instruction, None)
	}
	return pushOutcome(frame, instruction, result)
}

// prepareReraiseStar separates newly raised exceptions from subgroups carrying
// the original metadata, projects reraised leaves, and combines both sets.
func prepareReraiseStar(original *Exception, results []Value) *Exception {
	if original.group == nil {
		var exceptions []*Exception
		for _, result := range results {
			if exception, ok := result.(*Exception); ok {
				exceptions = append(exceptions, exception)
			}
		}
		return combineExceptionStarResults(exceptions)
	}

	var raisedExceptions []*Exception
	reraisedLeaves := make(map[*Exception]struct{})
	for _, result := range results {
		exception, ok := result.(*Exception)
		if !ok {
			continue
		}
		if sameExceptionMetadata(exception, original) {
			collectExceptionLeaves(exception, reraisedLeaves)
		} else {
			raisedExceptions = append(raisedExceptions, exception)
		}
	}
	if projected := projectExceptionLeaves(original, reraisedLeaves); projected != nil {
		raisedExceptions = append(raisedExceptions, projected)
	}
	return combineExceptionStarResults(raisedExceptions)
}

// sameExceptionMetadata identifies derived subgroups by the origin, chain, and
// traceback fields copied from the initially raised exception group.
func sameExceptionMetadata(left, right *Exception) bool {
	if left.cause != right.cause || left.context != right.context ||
		left.suppressContext != right.suppressContext ||
		left.originFrame != right.originFrame ||
		left.originInstruction != right.originInstruction ||
		len(left.traceback) != len(right.traceback) {
		return false
	}
	for index, entry := range left.traceback {
		if entry != right.traceback[index] {
			return false
		}
	}
	return true
}

func collectExceptionLeaves(exception *Exception, leaves map[*Exception]struct{}) {
	if exception.group == nil {
		leaves[exception] = struct{}{}
		return
	}
	for _, child := range exception.group.elements {
		collectExceptionLeaves(child.(*Exception), leaves)
	}
}

func projectExceptionLeaves(
	exception *Exception,
	leaves map[*Exception]struct{},
) *Exception {
	if exception.group == nil {
		if _, keep := leaves[exception]; keep {
			return exception
		}
		return nil
	}
	children := make([]Value, 0, len(exception.group.elements))
	for _, child := range exception.group.elements {
		if projected := projectExceptionLeaves(child.(*Exception), leaves); projected != nil {
			children = append(children, projected)
		}
	}
	return deriveExceptionGroup(exception, children)
}

func combineExceptionStarResults(exceptions []*Exception) *Exception {
	if len(exceptions) == 0 {
		return nil
	}
	if len(exceptions) == 1 {
		return exceptions[0]
	}
	children := make([]Value, len(exceptions))
	for index, exception := range exceptions {
		children[index] = exception
	}
	return &Exception{
		class:   exceptionGroupTypeForChildren(children),
		message: "",
		group:   &tupleValue{elements: children},
	}
}
