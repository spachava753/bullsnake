package runtime

import "strconv"

// generatorState records whether a generator frame can be entered.
type generatorState uint8

const (
	generatorCreated generatorState = iota
	generatorRunning
	generatorSuspended
	generatorCompleted
)

type generatorResumeKind uint8

const (
	generatorIteration generatorResumeKind = iota
	generatorCall
	generatorClose
	generatorDelegate
	generatorDelegateClose
)

type generatorResume struct {
	kind         generatorResumeKind
	target       int
	instruction  int
	defaultValue Value
	hasDefault   bool
}

type generatorValue struct {
	frame         *frame
	qualifiedName string
	state         generatorState
	resume        generatorResume
}

type generatorSendMethod struct {
	generator *generatorValue
}

type generatorThrowMethod struct {
	generator *generatorValue
}

type generatorCloseMethod struct {
	generator *generatorValue
}

func (*generatorSendMethod) TypeName() string { return "builtin_function_or_method" }
func (method *generatorSendMethod) Repr() string {
	return "<built-in method send of " + method.generator.Repr() + ">"
}
func (*generatorSendMethod) isValue() {}

func (*generatorThrowMethod) TypeName() string { return "builtin_function_or_method" }
func (method *generatorThrowMethod) Repr() string {
	return "<built-in method throw of " + method.generator.Repr() + ">"
}
func (*generatorThrowMethod) isValue() {}

func (*generatorCloseMethod) TypeName() string { return "builtin_function_or_method" }
func (method *generatorCloseMethod) Repr() string {
	return "<built-in method close of " + method.generator.Repr() + ">"
}
func (*generatorCloseMethod) isValue() {}

func (*generatorValue) TypeName() string { return "generator" }
func (generator *generatorValue) Repr() string {
	return "<generator object " + generator.qualifiedName + ">"
}
func (*generatorValue) isValue() {}

func (generator *generatorValue) complete() {
	generator.state = generatorCompleted
	if generator.frame != nil {
		generator.frame.previous = nil
		generator.frame.generator = nil
		generator.frame = nil
	}
}

func transformGeneratorStopIteration(
	exception *Exception,
	generatorFrame *frame,
	instruction int,
) *Exception {
	if !isStopIteration(exception) {
		return exception
	}
	transformed := newException("RuntimeError", "generator raised StopIteration")
	transformed.cause = exception
	transformed.context = exception
	transformed.suppressContext = true
	transformed.originFrame = generatorFrame
	transformed.originInstruction = instruction
	transformed.traceback = append(transformed.traceback, tracebackEntry{
		frame:       generatorFrame,
		instruction: instruction,
	})
	return transformed
}

// executeBuiltinNext advances a native iterator synchronously or resumes a
// generator frame through the ordinary VM loop.
func executeBuiltinNext(
	caller *frame,
	instruction int,
	base int,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "next() takes no keyword arguments"),
		}, nil
	}
	if len(arguments) < 1 || len(arguments) > 2 {
		message := "next expected at least 1 argument, got 0"
		if len(arguments) > 2 {
			message = "next expected at most 2 arguments, got " + strconv.Itoa(len(arguments))
		}
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", message),
		}, nil
	}
	iterator := arguments[0]
	defaultValue := None
	hasDefault := len(arguments) == 2
	if hasDefault {
		defaultValue = arguments[1]
	}

	switch iterator := iterator.(type) {
	case *generatorValue:
		discardCallSegment(caller, base)
		if iterator.state == generatorCompleted {
			if hasDefault {
				return pushOutcome(caller, instruction, defaultValue)
			}
			return instructionOutcome{
				kind:      raised,
				exception: newStopIteration(None),
			}, nil
		}
		return resumeGenerator(
			caller,
			instruction,
			iterator,
			None,
			generatorResume{
				kind:         generatorCall,
				instruction:  instruction,
				defaultValue: defaultValue,
				hasDefault:   hasDefault,
			},
		)
	case valueIterator:
		value, ok, exception := iterator.next()
		discardCallSegment(caller, base)
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if ok {
			return pushOutcome(caller, instruction, value)
		}
		if hasDefault {
			return pushOutcome(caller, instruction, defaultValue)
		}
		return instructionOutcome{
			kind:      raised,
			exception: newStopIteration(None),
		}, nil
	default:
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+iterator.TypeName()+"' object is not an iterator",
			),
		}, nil
	}
}

// executeGeneratorSendCall validates the bound method call and resumes its
// generator with the supplied yield-expression value.
func executeGeneratorSendCall(
	caller *frame,
	instruction int,
	base int,
	method *generatorSendMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "generator.send() takes no keyword arguments"),
		}, nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"generator.send() takes exactly one argument ("+
					strconv.Itoa(len(arguments))+" given)",
			),
		}, nil
	}
	value := arguments[0]
	generator := method.generator
	discardCallSegment(caller, base)
	if generator.state == generatorCompleted {
		return instructionOutcome{
			kind:      raised,
			exception: newStopIteration(None),
		}, nil
	}
	if generator.state == generatorCreated && value != None {
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"can't send non-None value to a just-started generator",
			),
		}, nil
	}
	return resumeGenerator(
		caller,
		instruction,
		generator,
		value,
		generatorResume{kind: generatorCall, instruction: instruction},
	)
}

// executeGeneratorThrowCall normalizes the requested exception and injects it
// at a suspended yield through the VM's ordinary exception router.
func executeGeneratorThrowCall(
	caller *frame,
	instruction int,
	base int,
	method *generatorThrowMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "throw() takes no keyword arguments"),
		}, nil
	}
	if len(arguments) < 1 || len(arguments) > 3 {
		message := "throw expected at least 1 argument, got 0"
		if len(arguments) > 3 {
			message = "throw expected at most 3 arguments, got " + strconv.Itoa(len(arguments))
		}
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", message),
		}, nil
	}
	injected, failure := normalizeGeneratorThrow(arguments)
	discardCallSegment(caller, base)
	if failure != nil {
		return instructionOutcome{kind: raised, exception: failure}, nil
	}
	generator := method.generator
	if generator.state != generatorRunning {
		injected.originFrame = nil
		injected.originInstruction = 0
	}
	switch generator.state {
	case generatorRunning:
		return instructionOutcome{
			kind:      raised,
			exception: newException("ValueError", "generator already executing"),
		}, nil
	case generatorCompleted:
		return instructionOutcome{kind: raised, exception: injected}, nil
	case generatorCreated:
		generator.complete()
		return instructionOutcome{kind: raised, exception: injected}, nil
	case generatorSuspended:
		generator.state = generatorRunning
		generator.resume = generatorResume{kind: generatorCall, instruction: instruction}
		generator.frame.previous = caller
		return instructionOutcome{kind: called, frame: generator.frame, exception: injected}, nil
	default:
		return instructionOutcome{}, caller.failure(instruction, "invalid generator state")
	}
}

// normalizeGeneratorThrow accepts an exception instance or class, validates the
// deprecated value and traceback arguments, and constructs the injected value.
func normalizeGeneratorThrow(arguments []Value) (*Exception, *Exception) {
	if len(arguments) == 3 && arguments[2] != None {
		return nil, newException("TypeError", "throw() third argument must be a traceback object")
	}
	invalidMessage := "exceptions must be classes or instances deriving from BaseException, not " +
		arguments[0].TypeName()
	if instance, ok := arguments[0].(*Exception); ok {
		if len(arguments) > 1 && arguments[1] != None {
			return nil, newException(
				"TypeError",
				"instance exception may not have a separate value",
			)
		}
		return instance, nil
	}
	if len(arguments) == 1 {
		return normalizeRaisedValue(arguments[0], invalidMessage)
	}
	value := arguments[1]
	switch class := arguments[0].(type) {
	case *exceptionTypeValue:
		if instance, ok := value.(*Exception); ok &&
			instance.class.isSubclassOf(class) {
			return instance, nil
		}
		if isExceptionGroupType(class) {
			return nil, exceptionGroupArityError(0)
		}
		exception := newExceptionOfType(class, exceptionMessage([]Value{value}))
		if class.isSubclassOf(stopIterationType) {
			exception.stopIterationValue = value
		}
		return exception, nil
	case *typeValue:
		if !class.isExceptionClass() {
			return nil, newException("TypeError", invalidMessage)
		}
		if instance, ok := value.(*Exception); ok &&
			instance.userClass != nil && instance.userClass.isSubclassOf(class) {
			return instance, nil
		}
		if _, hasInitializer := class.lookup("__init__"); hasInitializer {
			return nil, newException(
				"TypeError",
				"custom exception initializers are not supported",
			)
		}
		exception := newUserException(class, exceptionMessage([]Value{value}))
		if class.builtinExceptionBase().isSubclassOf(stopIterationType) {
			exception.stopIterationValue = value
		}
		return exception, nil
	default:
		return nil, newException("TypeError", invalidMessage)
	}
}

// executeGeneratorCloseCall starts cleanup by injecting GeneratorExit into a
// suspended generator, while new and completed generators close immediately.
func executeGeneratorCloseCall(
	caller *frame,
	instruction int,
	base int,
	method *generatorCloseMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "generator.close() takes no keyword arguments"),
		}, nil
	}
	if len(arguments) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"generator.close() takes no arguments ("+
					strconv.Itoa(len(arguments))+" given)",
			),
		}, nil
	}
	discardCallSegment(caller, base)
	generator := method.generator
	switch generator.state {
	case generatorRunning:
		return instructionOutcome{
			kind:      raised,
			exception: newException("ValueError", "generator already executing"),
		}, nil
	case generatorCreated:
		generator.complete()
		return pushOutcome(caller, instruction, None)
	case generatorCompleted:
		return pushOutcome(caller, instruction, None)
	case generatorSuspended:
		generator.state = generatorRunning
		generator.resume = generatorResume{kind: generatorClose, instruction: instruction}
		generator.frame.previous = caller
		return instructionOutcome{
			kind:      called,
			frame:     generator.frame,
			exception: newException("GeneratorExit", ""),
		}, nil
	default:
		return instructionOutcome{}, caller.failure(instruction, "invalid generator state")
	}
}

// executeSend advances one yield-from delegate. A yielded value falls through
// to YIELD_VALUE, while completion replaces the delegate with its return value
// and jumps to the expression exit.
func executeSend(
	frame *frame,
	instruction int,
	target int,
) (instructionOutcome, error) {
	if len(frame.stack) < 2 {
		return instructionOutcome{}, frame.failure(instruction, "operand stack underflow")
	}
	sent, _ := frame.pop()
	delegate := frame.stack[len(frame.stack)-1]
	state := &delegationState{
		sendInstruction:  instruction,
		yieldInstruction: instruction + 1,
		target:           target,
	}

	switch delegate := delegate.(type) {
	case *generatorValue:
		if delegate.state == generatorCompleted {
			return finishDelegation(frame, instruction, target, None)
		}
		if delegate.state == generatorCreated && sent != None {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					"can't send non-None value to a just-started generator",
				),
			}, nil
		}
		outcome, err := resumeGenerator(
			frame,
			instruction,
			delegate,
			sent,
			generatorResume{
				kind:        generatorDelegate,
				target:      target,
				instruction: instruction,
			},
		)
		if err == nil && outcome.kind == called {
			frame.delegation = state
		}
		return outcome, err
	case valueIterator:
		if sent != None {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"AttributeError",
					"'"+delegate.TypeName()+"' object has no attribute 'send'",
				),
			}, nil
		}
		value, ok, exception := delegate.next()
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !ok {
			return finishDelegation(frame, instruction, target, None)
		}
		frame.delegation = state
		return pushOutcome(frame, instruction, value)
	default:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"'"+delegate.TypeName()+"' object is not an iterator",
			),
		}, nil
	}
}

func finishDelegation(
	frame *frame,
	instruction int,
	target int,
	result Value,
) (instructionOutcome, error) {
	frame.pop()
	frame.delegation = nil
	if !frame.push(result) {
		return instructionOutcome{}, frame.failure(
			instruction,
			"operand stack overflow while returning delegate result",
		)
	}
	frame.instruction = target
	return instructionOutcome{kind: advance}, nil
}

// forwardDelegatedException redirects an injection at a yield-from suspension
// into a generator delegate. GeneratorExit closes the delegate first and stays
// pending for the outer generator.
func forwardDelegatedException(
	outer *frame,
	instruction int,
	exception *Exception,
) (*frame, int, *Exception, bool, error) {
	delegation := outer.delegation
	if delegation == nil || instruction != delegation.yieldInstruction {
		return nil, 0, exception, false, nil
	}
	if len(outer.stack) == 0 {
		return nil, 0, exception, false, outer.failure(
			instruction,
			"yield-from delegate is missing from the operand stack",
		)
	}
	delegate, ok := outer.stack[len(outer.stack)-1].(*generatorValue)
	if !ok {
		outer.delegation = nil
		return nil, 0, exception, false, nil
	}
	generatorExit := exception.class != nil &&
		exception.class.isSubclassOf(generatorExitType)
	if generatorExit && delegate.state == generatorCompleted {
		outer.delegation = nil
		return nil, 0, exception, false, nil
	}
	if delegate.state != generatorSuspended || delegate.frame == nil {
		return nil, 0, exception, false, outer.failure(
			instruction,
			"yield-from generator delegate is not suspended",
		)
	}
	outer.instruction = delegation.yieldInstruction
	resumeKind := generatorDelegate
	forwarded := exception
	if generatorExit {
		delegation.closeException = exception
		resumeKind = generatorDelegateClose
		forwarded = newException("GeneratorExit", "")
	}
	delegate.state = generatorRunning
	delegate.resume = generatorResume{
		kind:        resumeKind,
		target:      delegation.target,
		instruction: delegation.sendInstruction,
	}
	delegate.frame.previous = outer
	injectedAt := delegate.frame.instruction - 1
	if injectedAt < 0 {
		return nil, 0, exception, false, delegate.frame.failure(
			injectedAt,
			"delegated exception has no suspended instruction",
		)
	}
	return delegate.frame, injectedAt, forwarded, true, nil
}

// takeDelegatedClose verifies the outer frame still retains the closing
// delegate, removes its delegation state, and returns the saved exception.
func takeDelegatedClose(
	caller *frame,
	delegate *generatorValue,
	instruction int,
) (*Exception, error) {
	if caller == nil {
		return nil, delegate.frame.failure(instruction, "delegated close has no caller")
	}
	delegation := caller.delegation
	if delegation == nil || delegation.closeException == nil {
		return nil, caller.failure(instruction, "delegated close has no pending exception")
	}
	if len(caller.stack) == 0 || caller.stack[len(caller.stack)-1] != delegate {
		return nil, caller.failure(instruction, "closing delegate is not retained by caller")
	}
	pending := delegation.closeException
	caller.delegation = nil
	return pending, nil
}

// suspendGenerator verifies the active frame and its resumption contract,
// detaches the frame, and returns the yielded value to the caller.
func suspendGenerator(
	active *frame,
	instruction int,
	value Value,
) (*frame, *Exception, error) {
	generator := active.generator
	if generator == nil || generator.state != generatorRunning {
		return nil, nil, active.failure(instruction, "yield has no running generator")
	}
	caller := active.previous
	if caller == nil {
		return nil, nil, active.failure(instruction, "generator has no resuming caller")
	}
	if (generator.resume.kind == generatorIteration ||
		generator.resume.kind == generatorDelegate) &&
		(len(caller.stack) == 0 || caller.stack[len(caller.stack)-1] != generator) {
		return nil, nil, active.failure(
			instruction,
			"generator iterator is not retained by caller",
		)
	}
	active.previous = nil
	generator.state = generatorSuspended
	if generator.resume.kind == generatorClose ||
		generator.resume.kind == generatorDelegateClose {
		if generator.resume.kind == generatorDelegateClose {
			caller.delegation = nil
		}
		return caller, newException("RuntimeError", "generator ignored GeneratorExit"), nil
	}
	if !caller.push(value) {
		return nil, nil, caller.failure(
			generator.resume.instruction,
			"operand stack overflow while receiving generator yield",
		)
	}
	return caller, nil, nil
}

// finishGenerator applies the saved iteration, explicit call, or close
// completion behavior and releases the completed frame.
func finishGenerator(
	active *frame,
	instruction int,
	result Value,
) (*frame, *Exception, error) {
	generator := active.generator
	if generator == nil || generator.state != generatorRunning {
		return nil, nil, active.failure(instruction, "return has no running generator")
	}
	caller := active.previous
	if caller == nil {
		return nil, nil, active.failure(instruction, "generator has no resuming caller")
	}
	resume := generator.resume
	switch resume.kind {
	case generatorIteration:
		if len(caller.stack) == 0 || caller.stack[len(caller.stack)-1] != generator {
			return nil, nil, active.failure(
				instruction,
				"generator iterator is not retained by caller",
			)
		}
		caller.pop()
		generator.complete()
		caller.instruction = resume.target
		return caller, nil, nil
	case generatorCall:
		generator.complete()
		if !resume.hasDefault {
			return caller, newStopIteration(result), nil
		}
		if !caller.push(resume.defaultValue) {
			return nil, nil, caller.failure(
				resume.instruction,
				"operand stack overflow while returning next default",
			)
		}
		return caller, nil, nil
	case generatorClose:
		generator.complete()
		if !caller.push(result) {
			return nil, nil, caller.failure(
				resume.instruction,
				"operand stack overflow while returning close result",
			)
		}
		return caller, nil, nil
	case generatorDelegate:
		if len(caller.stack) == 0 || caller.stack[len(caller.stack)-1] != generator {
			return nil, nil, active.failure(
				instruction,
				"delegated generator is not retained by caller",
			)
		}
		caller.pop()
		generator.complete()
		caller.delegation = nil
		if !caller.push(result) {
			return nil, nil, caller.failure(
				resume.instruction,
				"operand stack overflow while returning delegate result",
			)
		}
		caller.instruction = resume.target
		return caller, nil, nil
	case generatorDelegateClose:
		pending, err := takeDelegatedClose(caller, generator, instruction)
		if err != nil {
			return nil, nil, err
		}
		generator.complete()
		return caller, pending, nil
	default:
		return nil, nil, active.failure(instruction, "invalid generator resumption")
	}
}

// resumeGeneratorIteration starts or resumes a generator for FOR_ITER, which
// retains the generator below each yielded value until exhaustion.
func resumeGeneratorIteration(
	caller *frame,
	instruction int,
	target int,
	generator *generatorValue,
) (instructionOutcome, error) {
	if generator.state == generatorCompleted {
		caller.pop()
		caller.instruction = target
		return instructionOutcome{kind: advance}, nil
	}
	return resumeGenerator(
		caller,
		instruction,
		generator,
		None,
		generatorResume{
			kind:        generatorIteration,
			target:      target,
			instruction: instruction,
		},
	)
}

// resumeGenerator attaches a created or suspended frame to the caller and
// records how the next yield or return must finish the suspended instruction.
func resumeGenerator(
	caller *frame,
	instruction int,
	generator *generatorValue,
	value Value,
	resume generatorResume,
) (instructionOutcome, error) {
	switch generator.state {
	case generatorRunning:
		return instructionOutcome{
			kind:      raised,
			exception: newException("ValueError", "generator already executing"),
		}, nil
	case generatorSuspended:
		if generator.frame == nil {
			return instructionOutcome{}, caller.failure(
				instruction,
				"suspended generator has no frame",
			)
		}
		if !generator.frame.push(value) {
			return instructionOutcome{}, generator.frame.failure(
				generator.frame.instruction,
				"operand stack overflow while resuming generator",
			)
		}
	case generatorCreated:
		if generator.frame == nil {
			return instructionOutcome{}, caller.failure(
				instruction,
				"created generator has no frame",
			)
		}
	case generatorCompleted:
		return instructionOutcome{}, caller.failure(instruction, "completed generator resumed")
	default:
		return instructionOutcome{}, caller.failure(instruction, "invalid generator state")
	}
	generator.state = generatorRunning
	generator.resume = resume
	generator.frame.previous = caller
	return instructionOutcome{kind: called, frame: generator.frame}, nil
}
