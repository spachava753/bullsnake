package runtime

import "strconv"

type asyncGeneratorWrappedValue struct {
	value Value
}

func (*asyncGeneratorWrappedValue) TypeName() string {
	return "async_generator_wrapped_value"
}
func (wrapped *asyncGeneratorWrappedValue) Repr() string { return wrapped.value.Repr() }
func (*asyncGeneratorWrappedValue) isValue()             {}

type asyncGeneratorNextState uint8

const (
	asyncGeneratorNextCreated asyncGeneratorNextState = iota
	asyncGeneratorNextActive
	asyncGeneratorNextClosed
)

type asyncGeneratorNextValue struct {
	generator *generatorValue
	state     asyncGeneratorNextState
	sendValue Value
}

type asyncGeneratorThrowValue struct {
	generator *generatorValue
	state     asyncGeneratorNextState
	arguments []Value
	close     bool
}

func (*asyncGeneratorThrowValue) TypeName() string { return "async_generator_athrow" }
func (*asyncGeneratorThrowValue) Repr() string     { return "<async_generator_athrow object>" }
func (*asyncGeneratorThrowValue) isValue()         {}

func (*asyncGeneratorNextValue) TypeName() string { return "async_generator_asend" }
func (*asyncGeneratorNextValue) Repr() string     { return "<async_generator_asend object>" }
func (*asyncGeneratorNextValue) isValue()         {}

type asyncGeneratorAIterMethod struct {
	generator *generatorValue
}

type asyncGeneratorANextMethod struct {
	generator *generatorValue
}

type asyncGeneratorASendMethod struct {
	generator *generatorValue
}

type asyncGeneratorAThrowMethod struct {
	generator *generatorValue
}

type asyncGeneratorACloseMethod struct {
	generator *generatorValue
}

func (*asyncGeneratorAIterMethod) TypeName() string { return "method-wrapper" }
func (method *asyncGeneratorAIterMethod) Repr() string {
	return "<method-wrapper '__aiter__' of " + method.generator.Repr() + ">"
}
func (*asyncGeneratorAIterMethod) isValue() {}

func (*asyncGeneratorANextMethod) TypeName() string { return "method-wrapper" }
func (method *asyncGeneratorANextMethod) Repr() string {
	return "<method-wrapper '__anext__' of " + method.generator.Repr() + ">"
}
func (*asyncGeneratorANextMethod) isValue() {}

func (*asyncGeneratorASendMethod) TypeName() string { return "builtin_function_or_method" }
func (method *asyncGeneratorASendMethod) Repr() string {
	return "<built-in method asend of " + method.generator.Repr() + ">"
}
func (*asyncGeneratorASendMethod) isValue() {}

func (*asyncGeneratorAThrowMethod) TypeName() string { return "builtin_function_or_method" }
func (method *asyncGeneratorAThrowMethod) Repr() string {
	return "<built-in method athrow of " + method.generator.Repr() + ">"
}
func (*asyncGeneratorAThrowMethod) isValue() {}

func (*asyncGeneratorACloseMethod) TypeName() string { return "builtin_function_or_method" }
func (method *asyncGeneratorACloseMethod) Repr() string {
	return "<built-in method aclose of " + method.generator.Repr() + ">"
}
func (*asyncGeneratorACloseMethod) isValue() {}

func executeAsyncGeneratorAIterCall(
	caller *frame,
	instruction int,
	base int,
	method *asyncGeneratorAIterMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if exception := validateAsyncGeneratorMethodCall(
		"__aiter__",
		arguments,
		keywords,
	); exception != nil {
		discardCallSegment(caller, base)
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, method.generator)
}

func executeAsyncGeneratorANextCall(
	caller *frame,
	instruction int,
	base int,
	method *asyncGeneratorANextMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if exception := validateAsyncGeneratorMethodCall(
		"__anext__",
		arguments,
		keywords,
	); exception != nil {
		discardCallSegment(caller, base)
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &asyncGeneratorNextValue{
		generator: method.generator,
		state:     asyncGeneratorNextCreated,
		sendValue: None,
	})
}

func executeAsyncGeneratorASendCall(
	caller *frame,
	instruction int,
	base int,
	method *asyncGeneratorASendMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"async_generator.asend() takes no keyword arguments",
			),
		}, nil
	}
	if len(arguments) != 1 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"TypeError",
				"async_generator.asend() takes exactly one argument ("+
					strconv.Itoa(len(arguments))+" given)",
			),
		}, nil
	}
	sendValue := arguments[0]
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &asyncGeneratorNextValue{
		generator: method.generator,
		state:     asyncGeneratorNextCreated,
		sendValue: sendValue,
	})
}

// executeAsyncGeneratorAThrowCall validates and copies exception arguments into
// a lazy awaitable so injection begins only when Python awaits the result.
func executeAsyncGeneratorAThrowCall(
	caller *frame,
	instruction int,
	base int,
	method *asyncGeneratorAThrowMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if keywords != nil && len(keywords.entries) != 0 {
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", "athrow() takes no keyword arguments"),
		}, nil
	}
	if len(arguments) < 1 || len(arguments) > 3 {
		message := "athrow expected at least 1 argument, got 0"
		if len(arguments) > 3 {
			message = "athrow expected at most 3 arguments, got " + strconv.Itoa(len(arguments))
		}
		discardCallSegment(caller, base)
		return instructionOutcome{
			kind:      raised,
			exception: newException("TypeError", message),
		}, nil
	}
	throwArguments := make([]Value, len(arguments))
	copy(throwArguments, arguments)
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &asyncGeneratorThrowValue{
		generator: method.generator,
		state:     asyncGeneratorNextCreated,
		arguments: throwArguments,
	})
}

func executeAsyncGeneratorACloseCall(
	caller *frame,
	instruction int,
	base int,
	method *asyncGeneratorACloseMethod,
	arguments []Value,
	keywords *dictValue,
) (instructionOutcome, error) {
	if exception := validateAsyncGeneratorMethodCall(
		"aclose",
		arguments,
		keywords,
	); exception != nil {
		discardCallSegment(caller, base)
		return instructionOutcome{kind: raised, exception: exception}, nil
	}
	discardCallSegment(caller, base)
	return pushOutcome(caller, instruction, &asyncGeneratorThrowValue{
		generator: method.generator,
		state:     asyncGeneratorNextCreated,
		close:     true,
	})
}

func validateAsyncGeneratorMethodCall(
	name string,
	arguments []Value,
	keywords *dictValue,
) *Exception {
	if keywords != nil && len(keywords.entries) != 0 {
		return newException(
			"TypeError",
			"async_generator."+name+"() takes no keyword arguments",
		)
	}
	if len(arguments) != 0 {
		return newException(
			"TypeError",
			"async_generator."+name+"() takes no arguments ("+
				strconv.Itoa(len(arguments))+" given)",
		)
	}
	return nil
}

// executeAsyncGeneratorNextSend resumes one __anext__ awaitable. A wrapped
// async yield completes the awaitable; an ordinary suspension remains active.
func executeAsyncGeneratorNextSend(
	caller *frame,
	instruction int,
	target int,
	awaitable *asyncGeneratorNextValue,
	sent Value,
) (instructionOutcome, error) {
	generator := awaitable.generator
	resumeValue := sent
	switch awaitable.state {
	case asyncGeneratorNextClosed:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"RuntimeError",
				"cannot reuse already awaited __anext__()/asend()",
			),
		}, nil
	case asyncGeneratorNextCreated:
		if generator.state == generatorCompleted {
			awaitable.state = asyncGeneratorNextClosed
			return instructionOutcome{
				kind:      raised,
				exception: newExceptionOfType(stopAsyncIterationType, ""),
			}, nil
		}
		if generator.state == generatorRunning {
			awaitable.state = asyncGeneratorNextClosed
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"RuntimeError",
					"anext(): asynchronous generator is already running",
				),
			}, nil
		}
		resumeValue = awaitable.sendValue
		awaitable.sendValue = None
		if generator.state == generatorCreated && resumeValue != None {
			awaitable.state = asyncGeneratorNextClosed
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"TypeError",
					"can't send non-None value to a just-started async generator",
				),
			}, nil
		}
		awaitable.state = asyncGeneratorNextActive
	case asyncGeneratorNextActive:
		if generator.state != generatorSuspended {
			return instructionOutcome{}, caller.failure(
				instruction,
				"active async generator awaitable has no suspended generator",
			)
		}
	default:
		return instructionOutcome{}, caller.failure(
			instruction,
			"invalid async generator awaitable state",
		)
	}
	return resumeGenerator(
		caller,
		instruction,
		generator,
		resumeValue,
		generatorResume{
			kind:        generatorAsyncNext,
			target:      target,
			instruction: instruction,
			asyncNext:   awaitable,
		},
	)
}

// executeAsyncGeneratorThrowSend starts exception injection or resumes an inner
// await while retaining the one-shot athrow awaitable on the caller's stack.
func executeAsyncGeneratorThrowSend(
	caller *frame,
	instruction int,
	target int,
	awaitable *asyncGeneratorThrowValue,
	sent Value,
) (instructionOutcome, error) {
	generator := awaitable.generator
	resume := generatorResume{
		kind:        generatorAsyncThrow,
		target:      target,
		instruction: instruction,
		asyncThrow:  awaitable,
	}
	switch awaitable.state {
	case asyncGeneratorNextClosed:
		return instructionOutcome{
			kind: raised,
			exception: newException(
				"RuntimeError",
				"cannot reuse already awaited aclose()/athrow()",
			),
		}, nil
	case asyncGeneratorNextCreated:
		if generator.state == generatorCompleted {
			awaitable.state = asyncGeneratorNextClosed
			return finishDelegation(caller, instruction, target, None)
		}
		if generator.state == generatorRunning {
			awaitable.state = asyncGeneratorNextClosed
			operation := "athrow"
			if awaitable.close {
				operation = "aclose"
			}
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"RuntimeError",
					operation+"(): asynchronous generator is already running",
				),
			}, nil
		}
		if sent != None {
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"RuntimeError",
					"can't send non-None value to a just-started coroutine",
				),
			}, nil
		}
		var injected *Exception
		var failure *Exception
		if awaitable.close {
			injected = newException("GeneratorExit", "")
		} else {
			injected, failure = normalizeGeneratorThrow(awaitable.arguments)
		}
		awaitable.arguments = nil
		if failure != nil {
			awaitable.state = asyncGeneratorNextClosed
			return instructionOutcome{kind: raised, exception: failure}, nil
		}
		awaitable.state = asyncGeneratorNextActive
		injected.originFrame = nil
		injected.originInstruction = 0
		if generator.state == generatorCreated {
			awaitable.state = asyncGeneratorNextClosed
			generator.complete()
			if awaitable.close {
				return finishDelegation(caller, instruction, target, None)
			}
			return instructionOutcome{kind: raised, exception: injected}, nil
		}
		if generator.state != generatorSuspended || generator.frame == nil {
			return instructionOutcome{}, caller.failure(
				instruction,
				"athrow target has no suspended async generator",
			)
		}
		generator.state = generatorRunning
		generator.resume = resume
		generator.frame.previous = caller
		return instructionOutcome{
			kind:      called,
			frame:     generator.frame,
			exception: injected,
		}, nil
	case asyncGeneratorNextActive:
		if generator.state != generatorSuspended {
			return instructionOutcome{}, caller.failure(
				instruction,
				"active athrow awaitable has no suspended async generator",
			)
		}
		return resumeGenerator(caller, instruction, generator, sent, resume)
	default:
		return instructionOutcome{}, caller.failure(
			instruction,
			"invalid async generator athrow awaitable state",
		)
	}
}

func completeAsyncGeneratorClose(
	active *frame,
	caller *frame,
	instruction int,
	target int,
	awaitable *asyncGeneratorThrowValue,
) error {
	if caller == nil || len(caller.stack) == 0 ||
		caller.stack[len(caller.stack)-1] != awaitable {
		return active.failure(
			instruction,
			"async generator close awaitable is not retained by caller",
		)
	}
	awaitable.state = asyncGeneratorNextClosed
	caller.pop()
	if !caller.push(None) {
		return caller.failure(
			instruction,
			"operand stack overflow while completing async generator close",
		)
	}
	caller.instruction = target
	return nil
}

// finishAsyncGeneratorYield closes one async-generator protocol awaitable,
// replaces it with the wrapped item, and leaves the frame suspended for later.
func finishAsyncGeneratorYield(
	active *frame,
	instruction int,
	wrapped *asyncGeneratorWrappedValue,
) (*frame, *Exception, error) {
	generator := active.generator
	resume := generator.resume
	caller := active.previous
	var awaitable Value
	closing := false
	switch resume.kind {
	case generatorAsyncNext:
		if resume.asyncNext == nil {
			return nil, nil, active.failure(
				instruction,
				"async generator yield has no next awaitable",
			)
		}
		resume.asyncNext.state = asyncGeneratorNextClosed
		awaitable = resume.asyncNext
	case generatorAsyncThrow:
		if resume.asyncThrow == nil {
			return nil, nil, active.failure(
				instruction,
				"async generator yield has no athrow awaitable",
			)
		}
		resume.asyncThrow.state = asyncGeneratorNextClosed
		awaitable = resume.asyncThrow
		closing = resume.asyncThrow.close
	default:
		return nil, nil, active.failure(
			instruction,
			"async generator yield has no protocol awaitable",
		)
	}
	if caller == nil || len(caller.stack) == 0 ||
		caller.stack[len(caller.stack)-1] != awaitable {
		return nil, nil, active.failure(
			instruction,
			"async generator awaitable is not retained by caller",
		)
	}
	active.previous = nil
	generator.state = generatorSuspended
	if closing {
		return caller, newException(
			"RuntimeError",
			"async generator ignored GeneratorExit",
		), nil
	}
	caller.pop()
	if !caller.push(wrapped.value) {
		return nil, nil, caller.failure(
			resume.instruction,
			"operand stack overflow while receiving async generator yield",
		)
	}
	caller.instruction = resume.target
	return caller, nil, nil
}

var _ Value = (*asyncGeneratorWrappedValue)(nil)
var _ Value = (*asyncGeneratorNextValue)(nil)
var _ Value = (*asyncGeneratorThrowValue)(nil)
var _ Value = (*asyncGeneratorAIterMethod)(nil)
var _ Value = (*asyncGeneratorANextMethod)(nil)
var _ Value = (*asyncGeneratorASendMethod)(nil)
var _ Value = (*asyncGeneratorAThrowMethod)(nil)
var _ Value = (*asyncGeneratorACloseMethod)(nil)
