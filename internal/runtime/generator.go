package runtime

// generatorState records whether a generator frame can be entered.
type generatorState uint8

const (
	generatorCreated generatorState = iota
	generatorRunning
	generatorSuspended
	generatorCompleted
)

type generatorValue struct {
	frame         *frame
	qualifiedName string
	state         generatorState
	resumeTarget  int
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

// suspendGenerator verifies the active frame and its retained iterator slot,
// detaches the frame, and returns the yielded value to the caller.
func suspendGenerator(active *frame, instruction int, value Value) (*frame, error) {
	generator := active.generator
	if generator == nil || generator.state != generatorRunning {
		return nil, active.failure(instruction, "yield has no running generator")
	}
	caller := active.previous
	if caller == nil || len(caller.stack) == 0 || caller.stack[len(caller.stack)-1] != generator {
		return nil, active.failure(instruction, "generator has no suspended iterator caller")
	}
	active.previous = nil
	generator.state = generatorSuspended
	if !caller.push(value) {
		return nil, caller.failure(
			caller.instruction-1,
			"operand stack overflow while receiving generator yield",
		)
	}
	return caller, nil
}

// finishGenerator validates the retained iterator, removes it from the caller,
// releases the completed frame, and takes FOR_ITER's exhaustion jump.
func finishGenerator(active *frame, instruction int) (*frame, error) {
	generator := active.generator
	if generator == nil || generator.state != generatorRunning {
		return nil, active.failure(instruction, "return has no running generator")
	}
	caller := active.previous
	if caller == nil || len(caller.stack) == 0 || caller.stack[len(caller.stack)-1] != generator {
		return nil, active.failure(instruction, "generator has no suspended iterator caller")
	}
	target := generator.resumeTarget
	caller.pop()
	generator.complete()
	caller.instruction = target
	return caller, nil
}

// resumeGenerator attaches a suspended generator frame to the current caller.
// FOR_ITER supplies None as the result of the previous yield expression.
func resumeGenerator(
	caller *frame,
	instruction int,
	target int,
	generator *generatorValue,
) (instructionOutcome, error) {
	switch generator.state {
	case generatorCompleted:
		caller.pop()
		caller.instruction = target
		return instructionOutcome{kind: advance}, nil
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
		if !generator.frame.push(None) {
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
	default:
		return instructionOutcome{}, caller.failure(instruction, "invalid generator state")
	}
	generator.state = generatorRunning
	generator.resumeTarget = target
	generator.frame.previous = caller
	return instructionOutcome{kind: called, frame: generator.frame}, nil
}
