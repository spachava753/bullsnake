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
	generatorNext
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
				kind:         generatorNext,
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

// suspendGenerator verifies the active frame and its resumption contract,
// detaches the frame, and returns the yielded value to the caller.
func suspendGenerator(active *frame, instruction int, value Value) (*frame, error) {
	generator := active.generator
	if generator == nil || generator.state != generatorRunning {
		return nil, active.failure(instruction, "yield has no running generator")
	}
	caller := active.previous
	if caller == nil {
		return nil, active.failure(instruction, "generator has no resuming caller")
	}
	if generator.resume.kind == generatorIteration &&
		(len(caller.stack) == 0 || caller.stack[len(caller.stack)-1] != generator) {
		return nil, active.failure(instruction, "generator iterator is not retained by caller")
	}
	active.previous = nil
	generator.state = generatorSuspended
	if !caller.push(value) {
		return nil, caller.failure(
			generator.resume.instruction,
			"operand stack overflow while receiving generator yield",
		)
	}
	return caller, nil
}

// finishGenerator applies the saved FOR_ITER or next completion behavior and
// releases the completed frame. It returns an exception only for bare next.
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
	case generatorNext:
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
