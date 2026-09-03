package runtime

import (
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

var coroutineRunnerCode = bytecode.NewCode(bytecode.CodeSpec{
	Name:          "<coroutine runner>",
	QualifiedName: "<coroutine runner>",
	StackSize:     1,
	Instructions: []bytecode.Instruction{
		{Opcode: bytecode.LoadConst},
		{Opcode: bytecode.AwaitValue},
		{Opcode: bytecode.ReturnValue},
	},
	Positions: make([]lexer.Span, 3),
	Constants: []bytecode.Constant{bytecode.None()},
})

type asyncioRunnerValue struct {
	runtime       *Runtime
	loop          *asyncioLoopValue
	customFactory bool
}

func (*asyncioRunnerValue) TypeName() string { return "Runner" }
func (*asyncioRunnerValue) Repr() string     { return "<asyncio.Runner>" }
func (*asyncioRunnerValue) isValue()         {}

// attribute exposes the small runner lifecycle used by IsolatedAsyncioTestCase.
func (runner *asyncioRunnerValue) attribute(name string) (Value, bool) {
	switch name {
	case "get_loop":
		return nativeFunctionNamed("Runner.get_loop", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return runner.getLoop(), nil, nil
			}), true
	case "run":
		return nativeKeywordAwareFunctionNamed("Runner.run", 1, 1,
			func(caller *frame, arguments []Value, keywords *dictValue) (Value, *Exception, error) {
				coroutine, ok := arguments[0].(*coroutineValue)
				if !ok {
					return nil, newException("TypeError", "an asyncio coroutine is required"), nil
				}
				runner.getLoop()
				previous := caller.runtime.currentContext
				if keywords != nil {
					context, found, exception := keywords.get(&stringValue{value: "context"})
					if exception != nil {
						return nil, exception, nil
					}
					if selected, ok := context.(*contextVarsValue); found && ok {
						caller.runtime.currentContext = selected
					}
				}
				defer func() { caller.runtime.currentContext = previous }()
				return runCoroutineSynchronously(caller, coroutine)
			}), true
	case "close":
		return nativeFunctionNamed("Runner.close", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				runner.loop = nil
				return None, nil, nil
			}), true
	default:
		return nil, false
	}
}

func (runtime *Runtime) context() *contextVarsValue {
	if runtime.currentContext == nil {
		runtime.currentContext = &contextVarsValue{}
	}
	if runtime.currentContext.values == nil {
		runtime.currentContext.values = make(map[*contextVarValue]Value)
	}
	return runtime.currentContext
}

func (runner *asyncioRunnerValue) getLoop() *asyncioLoopValue {
	if runner.loop == nil {
		runner.loop = &asyncioLoopValue{}
	}
	if !runner.customFactory {
		events := runner.runtime.newAsyncioEventsModule()
		if policy, _ := events.globals.get("_event_loop_policy"); policy == None {
			events.globals.values["_event_loop_policy"] = &asyncioPolicyValue{loop: runner.loop}
		}
	}
	return runner.loop
}

type asyncioLoopValue struct{}

func (*asyncioLoopValue) TypeName() string { return "EventLoop" }
func (*asyncioLoopValue) Repr() string     { return "<asyncio.EventLoop>" }
func (*asyncioLoopValue) isValue()         {}
func (*asyncioLoopValue) attribute(name string) (Value, bool) {
	switch name {
	case "create_future":
		return nativeFunctionNamed("EventLoop.create_future", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return &futureValue{}, nil, nil
			}), true
	case "stop", "close":
		return nativeFunctionNamed("EventLoop."+name, 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return None, nil, nil
			}), true
	default:
		return nil, false
	}
}

type futureValue struct {
	result Value
	done   bool
}

func (*futureValue) TypeName() string { return "Future" }
func (*futureValue) Repr() string     { return "<Future>" }
func (*futureValue) isValue()         {}
func (future *futureValue) attribute(name string) (Value, bool) {
	switch name {
	case "set_result":
		return nativeFunctionNamed("Future.set_result", 1, 1,
			func(_ *frame, arguments []Value) (Value, *Exception, error) {
				future.result = arguments[0]
				future.done = true
				return None, nil, nil
			}), true
	case "result":
		return nativeFunctionNamed("Future.result", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				if !future.done {
					return nil, newException("RuntimeError", "Result is not ready"), nil
				}
				return future.result, nil, nil
			}), true
	case "done":
		return nativeFunctionNamed("Future.done", 0, 0,
			func(_ *frame, _ []Value) (Value, *Exception, error) {
				return pythonBool(future.done), nil, nil
			}), true
	default:
		return nil, false
	}
}

var futureType = &typeValue{
	name: "Future", qualifiedName: "Future", module: "asyncio", namespace: newNamespace(),
	constructor: func(
		_ *typeValue, _ *frame, arguments []Value, _ *dictValue,
	) (Value, *Exception, error) {
		if len(arguments) != 0 {
			return nil, newException("TypeError", "Future() takes no positional arguments"), nil
		}
		return &futureValue{}, nil, nil
	},
}

type asyncioPolicyValue struct{ loop *asyncioLoopValue }

func (*asyncioPolicyValue) TypeName() string { return "_DefaultEventLoopPolicy" }
func (*asyncioPolicyValue) Repr() string     { return "<asyncio event loop policy>" }
func (*asyncioPolicyValue) isValue()         {}
func (policy *asyncioPolicyValue) attribute(name string) (Value, bool) {
	if name != "get_event_loop" {
		return nil, false
	}
	return nativeFunctionNamed("get_event_loop", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			if policy.loop == nil {
				policy.loop = &asyncioLoopValue{}
			}
			return policy.loop, nil, nil
		}), true
}

// newAsyncioModule builds the interpreter-local coroutine runner and task helpers.
func (runtime *Runtime) newAsyncioModule() *Module {
	if module, found := runtime.modules["asyncio"]; found {
		return module
	}
	module := newSystemModule("asyncio", "asyncio")
	module.isPackage = true
	runnerClass := newBootstrapClass("Runner", nil)
	runnerClass.module = "asyncio"
	runnerClass.constructor = func(
		_ *typeValue, _ *frame, _ []Value, keywords *dictValue,
	) (Value, *Exception, error) {
		customFactory := false
		if keywords != nil {
			if factory, found, exception := keywords.get(&stringValue{value: "loop_factory"}); exception != nil {
				return nil, exception, nil
			} else if found && factory != None {
				customFactory = true
			}
		}
		return &asyncioRunnerValue{runtime: runtime, customFactory: customFactory}, nil, nil
	}
	loopClass := newBootstrapClass("EventLoop", nil)
	loopClass.module = "asyncio"
	loopClass.constructor = func(
		_ *typeValue, _ *frame, _ []Value, _ *dictValue,
	) (Value, *Exception, error) {
		return &asyncioLoopValue{}, nil, nil
	}
	module.globals.values["Runner"] = runnerClass
	module.globals.values["EventLoop"] = loopClass
	module.globals.values["CancelledError"] = cancelledErrorType
	module.globals.values["Future"] = futureType
	setNativeFunction(module, "iscoroutine", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			_, ok := arguments[0].(*coroutineValue)
			return pythonBool(ok), nil, nil
		})
	setNativeFunction(module, "isfuture", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			_, ok := arguments[0].(*futureValue)
			return pythonBool(ok), nil, nil
		})
	setNativeFunction(module, "new_event_loop", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			return &asyncioLoopValue{}, nil, nil
		})
	setNativeKeywordFunction(module, "run", 1, 1,
		func(caller *frame, arguments []Value) (Value, *Exception, error) {
			coroutine, ok := arguments[0].(*coroutineValue)
			if !ok {
				return nil, newException("TypeError", "a coroutine was expected"), nil
			}
			return runCoroutineSynchronously(caller, coroutine)
		})
	setNativeFunction(module, "sleep", 1, 2, runtime.asyncioSleep)
	setNativeFunction(module, "create_task", 1, 2, runtime.asyncioCreateTask)
	setNativeFunction(module, "ensure_future", 1, 2, identityFirstArgument)
	setNativeFunction(module, "wait", 1, 1, runtime.asyncioWait)
	setNativeFunction(module, "set_event_loop", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			events := runtime.newAsyncioEventsModule()
			if policy, ok := events.globals.values["_event_loop_policy"].(*asyncioPolicyValue); ok {
				if loop, loopOK := arguments[0].(*asyncioLoopValue); loopOK {
					policy.loop = loop
				} else {
					policy.loop = nil
				}
			}
			return None, nil, nil
		})
	events := runtime.newAsyncioEventsModule()
	module.globals.values["events"] = events
	coroutines := newSystemModule("asyncio.coroutines", "asyncio")
	coroutines.globals.values["_is_coroutine"] = &objectValue{}
	runtime.cacheModule("asyncio.coroutines", coroutines)
	module.globals.values["coroutines"] = coroutines
	return module
}

func (runtime *Runtime) newAsyncioEventsModule() *Module {
	if module, found := runtime.modules["asyncio.events"]; found {
		return module
	}
	module := newSystemModule("asyncio.events", "asyncio")
	module.globals.values["_event_loop_policy"] = None
	setNativeFunction(module, "_set_event_loop_policy", 1, 1,
		func(_ *frame, arguments []Value) (Value, *Exception, error) {
			module.globals.values["_event_loop_policy"] = arguments[0]
			return None, nil, nil
		})
	setNativeFunction(module, "_get_event_loop_policy", 0, 0,
		func(_ *frame, _ []Value) (Value, *Exception, error) {
			if module.globals.values["_event_loop_policy"] == None {
				module.globals.values["_event_loop_policy"] = &asyncioPolicyValue{}
			}
			return module.globals.values["_event_loop_policy"], nil, nil
		})
	runtime.cacheModule("asyncio.events", module)
	return module
}

// constructContextVar validates a name and captures its optional default value.
func constructContextVar(
	_ *typeValue,
	_ *frame,
	arguments []Value,
	keywords *dictValue,
) (Value, *Exception, error) {
	if len(arguments) != 1 {
		return nil, newException("TypeError", "ContextVar() takes one positional argument"), nil
	}
	name, ok := arguments[0].(*stringValue)
	if !ok {
		return nil, newException("TypeError", "context variable name must be a str"), nil
	}
	variable := &contextVarValue{name: name.value}
	if keywords != nil {
		if value, found, exception := keywords.get(&stringValue{value: "default"}); exception != nil {
			return nil, exception, nil
		} else if found {
			variable.defaultVal = value
		}
	}
	return variable, nil, nil
}

func (runtime *Runtime) asyncioSleep(
	_ *frame,
	_ []Value,
) (Value, *Exception, error) {
	coroutine := &coroutineValue{name: "sleep", immediate: None}
	if runtime.cancellingTask {
		coroutine.immediate = nil
		coroutine.immediateException = newExceptionOfType(cancelledErrorType, "")
	}
	return coroutine, nil, nil
}

func (runtime *Runtime) asyncioCreateTask(
	caller *frame,
	arguments []Value,
) (Value, *Exception, error) {
	coroutine, ok := arguments[0].(*coroutineValue)
	if !ok {
		return nil, newException("TypeError", "a coroutine was expected"), nil
	}
	previous := runtime.cancellingTask
	runtime.cancellingTask = true
	_, _, err := runCoroutineSynchronously(caller, coroutine)
	runtime.cancellingTask = previous
	if err != nil {
		return nil, nil, err
	}
	return coroutine, nil, nil
}

// asyncioWait eagerly completes each selected awaitable and returns a completed wait result.
func (runtime *Runtime) asyncioWait(
	caller *frame,
	arguments []Value,
) (Value, *Exception, error) {
	iterator, exception, err := newIteratorForFrame(caller, arguments[0])
	if err != nil || exception != nil {
		return nil, exception, err
	}
	if iterator == nil {
		return nil, newException("TypeError", "expect a collection of awaitables"), nil
	}
	for {
		value, available, nextException, nextErr := nextNativeIterator(runtime, iterator)
		if nextErr != nil || nextException != nil {
			return nil, nextException, nextErr
		}
		if !available {
			break
		}
		if coroutine, ok := value.(*coroutineValue); ok && !coroutine.done {
			if _, runException, runErr := runCoroutineSynchronously(caller, coroutine); runErr != nil || runException != nil {
				return nil, runException, runErr
			}
		}
	}
	return &coroutineValue{name: "wait", immediate: &tupleValue{elements: []Value{
		&setValue{}, &setValue{},
	}}}, nil, nil
}

// runCoroutineSynchronously attaches a detached root coroutine to a validated collector frame.
func runCoroutineSynchronously(
	caller *frame,
	coroutine *coroutineValue,
) (Value, *Exception, error) {
	if coroutine.done {
		return nil, newException("RuntimeError", "cannot reuse already awaited coroutine"), nil
	}
	if coroutine.immediate != nil || coroutine.immediateException != nil {
		coroutine.done = true
		return coroutine.immediate, coroutine.immediateException, nil
	}
	prepared, err := caller.runtime.prepare(coroutineRunnerCode)
	if err != nil {
		return nil, nil, err
	}
	collector := &frame{
		runtime:         caller.runtime,
		code:            prepared,
		instruction:     1,
		stack:           []Value{coroutine},
		locals:          newNamespace(),
		globals:         caller.globals,
		builtins:        caller.builtins,
		logicalPrevious: caller,
	}
	result, unhandled, err := execute(&threadState{current: collector})
	if err != nil {
		return nil, nil, err
	}
	if unhandled != nil {
		return nil, unhandled.exception, nil
	}
	return result, nil, nil
}

var (
	_ Value          = (*asyncioRunnerValue)(nil)
	_ Value          = (*asyncioLoopValue)(nil)
	_ Value          = (*asyncioPolicyValue)(nil)
	_ Value          = (*contextVarValue)(nil)
	_ attributeValue = (*asyncioRunnerValue)(nil)
	_ attributeValue = (*asyncioPolicyValue)(nil)
	_ attributeValue = (*contextVarValue)(nil)
)
