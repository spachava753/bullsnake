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

// finishAsyncGeneratorYield closes one __anext__ awaitable, replaces it with
// the wrapped item, and leaves the async-generator frame suspended for later.
func finishAsyncGeneratorYield(
	active *frame,
	instruction int,
	wrapped *asyncGeneratorWrappedValue,
) (*frame, *Exception, error) {
	generator := active.generator
	resume := generator.resume
	caller := active.previous
	if resume.kind != generatorAsyncNext || resume.asyncNext == nil {
		return nil, nil, active.failure(
			instruction,
			"async generator yield has no next awaitable",
		)
	}
	if caller == nil || len(caller.stack) == 0 ||
		caller.stack[len(caller.stack)-1] != resume.asyncNext {
		return nil, nil, active.failure(
			instruction,
			"async generator next awaitable is not retained by caller",
		)
	}
	active.previous = nil
	generator.state = generatorSuspended
	resume.asyncNext.state = asyncGeneratorNextClosed
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
var _ Value = (*asyncGeneratorAIterMethod)(nil)
var _ Value = (*asyncGeneratorANextMethod)(nil)
var _ Value = (*asyncGeneratorASendMethod)(nil)
