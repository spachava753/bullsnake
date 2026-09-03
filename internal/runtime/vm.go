package runtime

import (
	"fmt"
	"strings"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type outcomeKind uint8

const (
	advance outcomeKind = iota
	called
	returned
	raised
	yielded
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
		case returned:
			if active.generator != nil {
				if err := finishGenerator(active, index); err != nil {
					return nil, nil, err
				}
				thread.current = active.previous
				active.previous = nil
				continue
			}
			if active.coroutine != nil {
				if err := finishCoroutine(active, index, outcome.value); err != nil {
					return nil, nil, err
				}
				thread.current = active.previous
				active.previous = nil
				continue
			}
			thread.current = active.previous
			result := outcome.value
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
				class := active.classBuild.finish(result).(*typeValue)
				result = class
				var hook Value
				var found bool
				if len(class.bases) != 0 {
					hook, found = class.bases[0].lookup("__init_subclass__")
				} else {
					hook, found = builtinTypeMethod("object", "__init_subclass__")
				}
				if found {
					_, exception, hookErr := callValueSynchronously(
						active, bindCallable(hook, class), nil,
					)
					if hookErr != nil {
						return nil, nil, hookErr
					}
					if exception != nil {
						caller := thread.current
						unhandled, routeErr := routeException(
							thread, caller, caller.instruction-1, exception, false,
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
			}
			if active.moduleImport != nil {
				loaded := active.moduleImport
				active.moduleImport = nil
				if separator := strings.LastIndexByte(loaded.module.name, '.'); separator >= 0 {
					parent := active.runtime.modules[loaded.module.name[:separator]]
					if parent == nil {
						return nil, nil, active.failure(index, "import parent is not cached")
					}
					parent.globals.values[loaded.module.name[separator+1:]] = loaded.module
				}
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
		case yielded:
			generator := active.generator
			if generator == nil || !generator.running || active.previous == nil {
				return nil, nil, active.failure(index, "yield outside a resumed generator")
			}
			caller := active.previous
			generator.running = false
			generator.needsResume = true
			active.previous = nil
			thread.current = caller
			if !caller.push(outcome.value) {
				return nil, nil, caller.failure(
					caller.instruction,
					"operand stack overflow while yielding to caller",
				)
			}
		default:
			return nil, nil, active.failure(index, "unknown execution outcome")
		}
	}
	return nil, nil, &BytecodeError{Instruction: -1, Message: "execution has no frame"}
}

// finishCoroutine replaces the awaitable retained by AWAIT_VALUE with the
// coroutine's return value and marks its detached frame complete.
func finishCoroutine(active *frame, index int, result Value) error {
	coroutine := active.coroutine
	caller := active.previous
	if coroutine == nil || !coroutine.running || caller == nil {
		return active.failure(index, "returned coroutine has no suspended caller")
	}
	instructionIndex := caller.instruction - 1
	if instructionIndex < 0 || instructionIndex >= len(caller.code.instructions) {
		return caller.failure(instructionIndex, "coroutine caller has no active instruction")
	}
	instruction := caller.code.instructions[instructionIndex]
	if instruction.Opcode != bytecode.AwaitValue || len(caller.stack) == 0 ||
		caller.stack[len(caller.stack)-1] != coroutine {
		return caller.failure(instructionIndex, "coroutine caller is not suspended at AWAIT_VALUE")
	}
	caller.pop()
	if !caller.push(result) {
		return caller.failure(instructionIndex, "operand stack overflow while returning from await")
	}
	coroutine.running = false
	coroutine.done = true
	return nil
}

// finishGenerator marks a returned generator exhausted and completes the
// suspended FOR_ITER edge in its caller without exposing the return value.
func finishGenerator(active *frame, index int) error {
	generator := active.generator
	caller := active.previous
	if generator == nil || !generator.running || caller == nil {
		return active.failure(index, "returned generator has no suspended caller")
	}
	instructionIndex := caller.instruction - 1
	if instructionIndex < 0 || instructionIndex >= len(caller.code.instructions) {
		return caller.failure(instructionIndex, "generator caller has no active instruction")
	}
	instruction := caller.code.instructions[instructionIndex]
	if instruction.Opcode != bytecode.ForIter || len(caller.stack) == 0 ||
		caller.stack[len(caller.stack)-1] != generator {
		return caller.failure(instructionIndex, "generator caller is not suspended at FOR_ITER")
	}
	caller.pop()
	caller.instruction = int(instruction.Operand)
	generator.running = false
	generator.done = true
	return nil
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
	if exception.originFrame == nil {
		exception.chainContext(activeHandledException(origin, instruction))
		exception.originFrame = origin
		exception.originInstruction = instruction
	}
	unhandled := &raisedOutcome{
		exception:   exception,
		frame:       exception.originFrame,
		instruction: exception.originInstruction,
	}
	current := origin
	currentInstruction := instruction
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

		for index := range current.stack {
			current.stack[index] = nil
		}
		current.discardImportedModule()
		caller := current.previous
		if current.generator != nil {
			current.generator.running = false
			current.generator.done = true
			current.previous = nil
		}
		if current.coroutine != nil {
			current.coroutine.running = false
			current.coroutine.done = true
			current.previous = nil
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
	case bytecode.YieldValue:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		return instructionOutcome{kind: yielded, value: value}, nil
	case bytecode.ExceptionType:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		exception, ok := value.(*Exception)
		if !ok {
			return instructionOutcome{}, frame.failure(index, "EXCEPTION_TYPE value is not an exception")
		}
		var exceptionClass Value = exception.class
		if exception.userClass != nil {
			exceptionClass = exception.userClass
		}
		return pushOutcome(frame, index, exceptionClass)
	case bytecode.MatchSequence:
		return executeMatchSequence(frame, index, instruction.Operand)
	case bytecode.MatchMapping:
		return executeMatchMapping(frame, index, instruction.Operand != 0)
	case bytecode.MatchClass:
		return executeMatchClass(frame, index, int(instruction.Operand))
	case bytecode.AwaitValue:
		if len(frame.stack) == 0 {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		awaited := frame.stack[len(frame.stack)-1]
		coroutine, ok := awaited.(*coroutineValue)
		if !ok {
			return instructionOutcome{
				kind:      raised,
				exception: newException("TypeError", "object "+awaited.TypeName()+" can't be used in 'await' expression"),
			}, nil
		}
		if coroutine.done {
			return instructionOutcome{
				kind:      raised,
				exception: newException("RuntimeError", "cannot reuse already awaited coroutine"),
			}, nil
		}
		if coroutine.immediate != nil || coroutine.immediateException != nil {
			frame.pop()
			coroutine.done = true
			if coroutine.immediateException != nil {
				return instructionOutcome{kind: raised, exception: coroutine.immediateException}, nil
			}
			return pushOutcome(frame, index, coroutine.immediate)
		}
		if coroutine.running {
			return instructionOutcome{
				kind:      raised,
				exception: newException("ValueError", "coroutine already executing"),
			}, nil
		}
		coroutine.running = true
		coroutine.frame.previous = frame
		return instructionOutcome{kind: called, frame: coroutine.frame}, nil
	case bytecode.ConvertValue:
		return executeConvertValue(frame, index, instruction.Operand)
	case bytecode.FormatSimple:
		return executeFormatSimple(frame, index)
	case bytecode.FormatWithSpec:
		return executeFormatWithSpec(frame, index)
	case bytecode.BuildString:
		return executeBuildString(frame, index, int(instruction.Operand))
	case bytecode.LoadNotImplementedError:
		return pushOutcome(frame, index, newException("NotImplementedError", ""))
	case bytecode.LoadAssertionError:
		return pushOutcome(frame, index, assertionErrorType)
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
	case bytecode.LoadLocals:
		return pushOutcome(frame, index, &namespaceValue{namespace: frame.locals})
	case bytecode.LoadFromDictOrGlobals:
		name := frame.code.names[instruction.Operand]
		value, found, exception, err := popClassNamespaceValue(frame, index, name)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !found {
			value, found = frame.globals.get(name)
			if !found {
				value, found = frame.builtins.get(name)
			}
		}
		if !found {
			return instructionOutcome{
				kind:      raised,
				exception: newException("NameError", fmt.Sprintf("name '%s' is not defined", name)),
			}, nil
		}
		return pushOutcome(frame, index, value)
	case bytecode.LoadFromDictOrDeref:
		derefIndex := int(instruction.Operand)
		var name string
		if derefIndex < len(frame.code.cells) {
			name = frame.code.cells[derefIndex]
		} else {
			name = frame.code.freeVars[derefIndex-len(frame.code.cells)]
		}
		value, found, exception, err := popClassNamespaceValue(frame, index, name)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if !found {
			value = frame.deref[derefIndex].value
			if value == nil {
				return instructionOutcome{
					kind:      raised,
					exception: unboundDerefException(frame.code, derefIndex),
				}, nil
			}
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
	case bytecode.LoadAttr:
		owner, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		name := frame.code.names[instruction.Operand]
		if class, ok := owner.(*typeValue); name == "__annotations__" && ok {
			value, exception, err := materializeClassAnnotations(frame, class)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return pushOutcome(frame, index, value)
		}
		if name == "__class__" {
			instance, isInstance := owner.(*instanceValue)
			attribute, hasOverride := Value(nil), false
			if isInstance {
				attribute, hasOverride = instance.class.lookup(name)
				if descriptor, ok := attribute.(*descriptorValue); !ok || descriptor.kind != propertyDescriptor {
					hasOverride = false
				}
			}
			if !hasOverride {
				return pushOutcome(frame, index, runtimeTypeOf(owner))
			}
		}
		if _, ok := owner.(valueIterator); ok {
			if value, found := directAttribute(owner, name); found {
				return pushOutcome(frame, index, value)
			}
		}
		switch owner := owner.(type) {
		case attributeValue:
			value, found := owner.attribute(name)
			if !found {
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
					),
				}, nil
			}
			return pushOutcome(frame, index, value)
		case *Module:
			if name == "__dict__" {
				return pushOutcome(frame, index, &namespaceValue{namespace: owner.globals})
			}
			value, found := owner.globals.get(name)
			if !found {
				if fallback, fallbackFound := owner.globals.get("__getattr__"); fallbackFound {
					value, exception, err := callValueSynchronously(
						frame, fallback, []Value{&stringValue{value: name}},
					)
					if err != nil {
						return instructionOutcome{}, err
					}
					if exception != nil {
						return instructionOutcome{kind: raised, exception: exception}, nil
					}
					return pushOutcome(frame, index, value)
				}
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"module '"+owner.name+"' has no attribute '"+name+"'",
					),
				}, nil
			}
			return pushOutcome(frame, index, value)
		case *typeValue:
			if value, found := intrinsicTypeAttribute(owner, name); found {
				return pushOutcome(frame, index, value)
			}
			value, found, fromMetaclass := lookupTypeAttribute(owner, name)
			if !found {
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"type object '"+owner.name+"' has no attribute '"+name+"'",
					),
				}, nil
			}
			if fromMetaclass {
				return pushOutcome(frame, index, bindCallable(value, owner))
			}
			if resolved, descriptor, exception, err := resolvePythonDescriptor(
				frame, value, None, owner,
			); descriptor {
				if err != nil {
					return instructionOutcome{}, err
				}
				if exception != nil {
					return instructionOutcome{kind: raised, exception: exception}, nil
				}
				return pushOutcome(frame, index, resolved)
			}
			return pushOutcome(frame, index, bindClassAttribute(value, owner))
		case *instanceValue:
			if name == "__dict__" {
				return pushOutcome(frame, index, &namespaceValue{namespace: owner.attributes})
			}
			value, found := owner.attributes.get(name)
			fromClass := !found
			if !found {
				value, found = owner.class.lookup(name)
			}
			if !found && owner.sequence != nil {
				value, found = listMethod(owner.sequence, name)
				fromClass = false
			}
			if !found && owner.mapping != nil {
				value, found = owner.mapping.attribute(name)
				fromClass = false
			}
			if !found {
				if fallback, fallbackFound := owner.class.lookup("__getattr__"); fallbackFound {
					value, exception, err := callValueSynchronously(
						frame,
						bindCallable(fallback, owner),
						[]Value{&stringValue{value: name}},
					)
					if err != nil {
						return instructionOutcome{}, err
					}
					if exception != nil {
						return instructionOutcome{kind: raised, exception: exception}, nil
					}
					return pushOutcome(frame, index, value)
				}
				return instructionOutcome{
					kind: raised,
					exception: newException(
						"AttributeError",
						"'"+owner.class.name+"' object has no attribute '"+name+"'",
					),
				}, nil
			}
			if fromClass {
				if resolved, descriptor, exception, err := resolvePythonDescriptor(
					frame, value, owner, owner.class,
				); descriptor {
					if err != nil {
						return instructionOutcome{}, err
					}
					if exception != nil {
						return instructionOutcome{kind: raised, exception: exception}, nil
					}
					return pushOutcome(frame, index, resolved)
				}
				switch descriptor := value.(type) {
				case *functionValue:
					value = &boundMethodValue{function: descriptor, self: owner}
				case *nativeFunctionValue:
					value = bindCallable(descriptor, owner)
				case *descriptorValue:
					switch descriptor.kind {
					case classMethodDescriptor:
						value = bindDescriptorCallable(descriptor.callable, owner.class)
					case staticMethodDescriptor:
						value = descriptor.callable
					case propertyDescriptor:
						return executeFunctionCall(
							frame,
							index,
							len(frame.stack),
							descriptor.callable,
							[]Value{owner},
							nil,
						)
					}
				case *memberDescriptorValue:
					stored, found := owner.attributes.get(descriptor.name)
					if !found {
						return instructionOutcome{kind: raised, exception: newException(
							"AttributeError", "'"+owner.class.name+"' object has no attribute '"+name+"'",
						)}, nil
					}
					value = stored
				}
			}
			return pushOutcome(frame, index, value)
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
		switch owner := owner.(type) {
		case *Module:
			owner.globals.values[name] = value
		case *typeValue:
			setTypeAttribute(owner, name, value)
		case *instanceValue:
			_, exception, err := setInstanceAttribute(frame, owner, name, value)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
		case *functionValue:
			setFunctionAttribute(owner, name, value)
		case *nativeFunctionValue:
			if owner.attributes == nil {
				owner.attributes = newNamespace()
			}
			owner.attributes.values[name] = value
		case *descriptorValue:
			if owner.attributes == nil {
				owner.attributes = newNamespace()
			}
			owner.attributes.values[name] = value
		case *weakReferenceValue:
			owner.attributes.values[name] = value
		case *fileValue:
			if owner.attributes == nil {
				owner.attributes = newNamespace()
			}
			owner.attributes.values[name] = value
		case *Exception:
			owner.setAttribute(name, value)
		case *tracebackValue:
			if name != "tb_next" || (value != None && value.TypeName() != "traceback") {
				return instructionOutcome{kind: raised, exception: newException(
					"TypeError", "traceback attribute is not writable",
				)}, nil
			}
			owner.next = value
		case *builtinTypeValue, *exceptionTypeValue:
			return instructionOutcome{kind: raised, exception: newException(
				"TypeError", "cannot set attributes of built-in/extension type",
			)}, nil
		default:
			return instructionOutcome{
				kind: raised,
				exception: newException(
					"AttributeError",
					"'"+owner.TypeName()+"' object has no attribute '"+name+"'",
				),
			}, nil
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.DeleteAttr:
		owner, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		name := frame.code.names[instruction.Operand]
		var attributes *Namespace
		missingMessage := "'" + owner.TypeName() + "' object has no attribute '" + name + "'"
		switch owner := owner.(type) {
		case *Module:
			attributes = owner.globals
			missingMessage = "module '" + owner.name + "' has no attribute '" + name + "'"
		case *typeValue:
			attributes = owner.namespace
			missingMessage = "type object '" + owner.name + "' has no attribute '" + name + "'"
		case *instanceValue:
			_, exception, err := deleteInstanceAttribute(frame, owner, name)
			if err != nil {
				return instructionOutcome{}, err
			}
			if exception != nil {
				return instructionOutcome{kind: raised, exception: exception}, nil
			}
			return instructionOutcome{kind: advance}, nil
		}
		if attributes == nil {
			return instructionOutcome{
				kind:      raised,
				exception: newException("AttributeError", missingMessage),
			}, nil
		}
		if _, found := attributes.values[name]; !found {
			return instructionOutcome{
				kind:      raised,
				exception: newException("AttributeError", missingMessage),
			}, nil
		}
		delete(attributes.values, name)
		return instructionOutcome{kind: advance}, nil
	case bytecode.StoreName:
		value, ok := frame.pop()
		if !ok {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		name := frame.code.names[instruction.Operand]
		if _, found := frame.locals.values[name]; !found {
			frame.locals.order = append(frame.locals.order, name)
		}
		frame.locals.values[name] = value
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
		takeJump, exception, err := truthValueForFrame(frame, value)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if instruction.Opcode == bytecode.PopJumpIfFalse {
			takeJump = !takeJump
		}
		if takeJump {
			frame.instruction = int(instruction.Operand)
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.JumpIfFalseOrPop, bytecode.JumpIfTrueOrPop:
		if len(frame.stack) == 0 {
			return instructionOutcome{}, frame.failure(index, "operand stack underflow")
		}
		value := frame.stack[len(frame.stack)-1]
		takeJump, exception, err := truthValueForFrame(frame, value)
		if err != nil {
			return instructionOutcome{}, err
		}
		if exception != nil {
			return instructionOutcome{kind: raised, exception: exception}, nil
		}
		if instruction.Opcode == bytecode.JumpIfFalseOrPop {
			takeJump = !takeJump
		}
		if takeJump {
			frame.instruction = int(instruction.Operand)
		} else {
			frame.pop()
		}
		return instructionOutcome{kind: advance}, nil
	case bytecode.BuildTuple:
		return executeBuildSequence(frame, index, int(instruction.Operand), true)
	case bytecode.BuildList:
		return executeBuildSequence(frame, index, int(instruction.Operand), false)
	case bytecode.BuildSet:
		return executeBuildSet(frame, index, int(instruction.Operand))
	case bytecode.BuildMap:
		return executeBuildMap(frame, index, int(instruction.Operand))
	case bytecode.MapSet:
		return executeMapSet(frame, index, int(instruction.Operand))
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
		code := frame.code.children[instruction.Operand]
		function := &functionValue{
			code:    code,
			globals: frame.globals,
			doc:     functionDoc(code),
		}
		return pushOutcome(frame, index, function)
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
		return executeSetAdd(frame, index, int(instruction.Operand))
	case bytecode.SetUpdate:
		return executeSetUpdate(frame, index)
	case bytecode.ListAppend:
		return executeListAppend(frame, index, int(instruction.Operand))
	case bytecode.ListExtend:
		return executeListExtend(frame, index)
	case bytecode.ListToTuple:
		return executeListToTuple(frame, index)
	case bytecode.BuildSlice:
		return executeBuildSlice(frame, index, int(instruction.Operand))
	case bytecode.GetIter:
		return executeGetIter(frame, index)
	case bytecode.ForIter:
		return executeForIter(frame, index, int(instruction.Operand))
	case bytecode.GetAIter:
		return executeGetAIter(frame, index)
	case bytecode.AsyncForIter:
		return executeAsyncForIter(frame, index, int(instruction.Operand))
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
