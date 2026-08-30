package compiler

import (
	"slices"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type instructionExceptionHandler struct {
	target     *jumpLabel
	stackDepth int
}

type exceptionHandlerCleanup struct {
	name string
	span lexer.Span
}

type controlCleanupKind uint8

const (
	exceptionHandlerControlCleanup controlCleanupKind = iota
	finallyControlCleanup
)

type controlCleanup struct {
	kind         controlCleanupKind
	handlerDepth int
	exception    exceptionHandlerCleanup
	finalBody    []compilerast.Stmt
}

// compileRaiseStatement evaluates an optional exception and cause before
// terminating the current path with the matching raise operand count.
func (compiler *compilerState) compileRaiseStatement(statement *compilerast.RaiseStmt) error {
	arguments := uint32(0)
	if statement.Exception != nil {
		if err := compiler.compileExpr(statement.Exception); err != nil {
			return err
		}
		arguments = 1
	}
	if statement.Cause != nil {
		if statement.Exception == nil {
			return compiler.error(statement.Span(), "raise cause has no exception")
		}
		if err := compiler.compileExpr(statement.Cause); err != nil {
			return err
		}
		arguments = 2
	}
	return compiler.emitTerminator(bytecode.RaiseVarargs, arguments, statement.Span())
}

// compileAssertStatement keeps message evaluation on the failing edge and
// rejoins the true edge after a terminating AssertionError raise.
func (compiler *compilerState) compileAssertStatement(statement *compilerast.AssertStmt) error {
	end := compiler.newLabel()
	if err := compiler.compileExpr(statement.Condition); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.PopJumpIfTrue, end, statement.Condition.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LoadAssertionError, 0, statement.Span()); err != nil {
		return err
	}
	if statement.Message != nil {
		if err := compiler.compileExpr(statement.Message); err != nil {
			return err
		}
		if err := compiler.emit(bytecode.Call, 1, statement.Span()); err != nil {
			return err
		}
	}
	if err := compiler.emitTerminator(bytecode.RaiseVarargs, 1, statement.Span()); err != nil {
		return err
	}
	return compiler.markLabel(end, statement.Span())
}

// compileTryStatement emits ordered ordinary handlers while keeping their
// dispatch and bodies outside the statement's own protected range.
func (compiler *compilerState) compileTryStatement(statement *compilerast.TryStmt) error {
	if len(statement.Finally) != 0 {
		return compiler.compileTryFinally(statement)
	}
	if len(statement.Handlers) == 0 {
		return compiler.error(statement.Span(), "try statement has no exception handlers")
	}
	for index, handler := range statement.Handlers {
		if handler.Star {
			return compiler.error(handler.Range, "exception-group handlers are not compiled")
		}
		if handler.Type == nil && index != len(statement.Handlers)-1 {
			return compiler.error(handler.Range, "bare exception handler is not last")
		}
	}

	baseDepth := compiler.stackDepth
	target := compiler.newLabel()
	end := compiler.newLabel()
	compiler.activeHandlers = append(compiler.activeHandlers, instructionExceptionHandler{
		target:     target,
		stackDepth: baseDepth,
	})
	err := compiler.compileStatements(statement.Body)
	compiler.activeHandlers = compiler.activeHandlers[:len(compiler.activeHandlers)-1]
	if err != nil {
		return err
	}
	if compiler.reachable {
		if err := compiler.compileStatements(statement.Else); err != nil {
			return err
		}
	}
	if compiler.reachable {
		if err := compiler.emitJump(bytecode.Jump, end, statement.Span()); err != nil {
			return err
		}
	}

	firstHandler := statement.Handlers[0]
	if err := compiler.mergeLabelDepth(target, baseDepth+1, firstHandler.Range); err != nil {
		return err
	}
	if baseDepth+1 > compiler.maxStack {
		compiler.maxStack = baseDepth + 1
	}
	if err := compiler.markLabel(target, firstHandler.Range); err != nil {
		return err
	}
	for _, handler := range statement.Handlers {
		var next *jumpLabel
		if handler.Type != nil {
			next = compiler.newLabel()
			if err := compiler.compileExpr(handler.Type); err != nil {
				return err
			}
			if err := compiler.emit(bytecode.CheckExceptionMatch, 0, handler.Range); err != nil {
				return err
			}
			if err := compiler.emitJump(bytecode.PopJumpIfFalse, next, handler.Range); err != nil {
				return err
			}
		}

		var cleanupTarget *jumpLabel
		var cleanup exceptionHandlerCleanup
		cleanupHandlerDepth := 0
		if handler.Name != "" {
			if err := compiler.emit(bytecode.Copy, 1, handler.Range); err != nil {
				return err
			}
			cleanup = exceptionHandlerCleanup{
				name: handler.Name,
				span: handler.Range,
			}
			cleanupHandlerDepth = len(compiler.activeHandlers)
			cleanupTarget = compiler.newLabel()
		}
		if err := compiler.emitLabelOperand(bytecode.EnterExcept, end, handler.Range); err != nil {
			return err
		}
		if handler.Name != "" {
			if err := compiler.emitNameStore(handler.Name, handler.Range); err != nil {
				return err
			}
			compiler.activeHandlers = append(compiler.activeHandlers, instructionExceptionHandler{
				target:     cleanupTarget,
				stackDepth: baseDepth,
			})
			compiler.controlCleanups = append(compiler.controlCleanups, controlCleanup{
				kind:         exceptionHandlerControlCleanup,
				handlerDepth: cleanupHandlerDepth,
				exception:    cleanup,
			})
		}
		err := compiler.compileStatements(handler.Body)
		if handler.Name != "" {
			compiler.controlCleanups = compiler.controlCleanups[:len(compiler.controlCleanups)-1]
			compiler.activeHandlers = compiler.activeHandlers[:len(compiler.activeHandlers)-1]
		}
		if err != nil {
			return err
		}
		if compiler.reachable {
			if handler.Name == "" {
				if err := compiler.emit(bytecode.LeaveExcept, 0, handler.Range); err != nil {
					return err
				}
			} else if err := compiler.emitExceptionHandlerCleanup(cleanup); err != nil {
				return err
			}
		}

		if cleanupTarget != nil {
			if compiler.reachable {
				if err := compiler.emitJump(bytecode.Jump, end, handler.Range); err != nil {
					return err
				}
			}
			if err := compiler.mergeLabelDepth(cleanupTarget, baseDepth+1, handler.Range); err != nil {
				return err
			}
			if baseDepth+1 > compiler.maxStack {
				compiler.maxStack = baseDepth + 1
			}
			if err := compiler.markLabel(cleanupTarget, handler.Range); err != nil {
				return err
			}
			if err := compiler.emitExceptionHandlerCleanup(cleanup); err != nil {
				return err
			}
			if err := compiler.emitTerminator(bytecode.Reraise, 0, handler.Range); err != nil {
				return err
			}
		}

		if next == nil {
			return compiler.markLabel(end, statement.Span())
		}
		if cleanupTarget == nil && compiler.reachable {
			if err := compiler.emitJump(bytecode.Jump, end, handler.Range); err != nil {
				return err
			}
		}
		if err := compiler.markLabel(next, handler.Range); err != nil {
			return err
		}
	}
	if compiler.reachable {
		if err := compiler.emitTerminator(bytecode.Reraise, 0, statement.Span()); err != nil {
			return err
		}
	}
	return compiler.markLabel(end, statement.Span())
}

// compileTryFinally duplicates the final suite for normal fallthrough and for
// an exceptional entry that keeps the pending exception below temporary values.
func (compiler *compilerState) compileTryFinally(statement *compilerast.TryStmt) error {
	if len(statement.Handlers) != 0 || len(statement.Else) != 0 {
		return compiler.error(statement.Span(), "combined try/except/finally is not compiled")
	}
	baseDepth := compiler.stackDepth
	handler := compiler.newLabel()
	end := compiler.newLabel()
	handlerDepth := len(compiler.activeHandlers)
	compiler.activeHandlers = append(compiler.activeHandlers, instructionExceptionHandler{
		target:     handler,
		stackDepth: baseDepth,
	})
	compiler.controlCleanups = append(compiler.controlCleanups, controlCleanup{
		kind:         finallyControlCleanup,
		handlerDepth: handlerDepth,
		finalBody:    statement.Finally,
	})
	compiler.finallyDepth++
	err := compiler.compileStatements(statement.Body)
	compiler.finallyDepth--
	compiler.controlCleanups = compiler.controlCleanups[:len(compiler.controlCleanups)-1]
	compiler.activeHandlers = compiler.activeHandlers[:len(compiler.activeHandlers)-1]
	if err != nil {
		return err
	}
	if compiler.reachable {
		compiler.finallyDepth++
		err = compiler.compileStatements(statement.Finally)
		compiler.finallyDepth--
		if err != nil {
			return err
		}
	}
	if compiler.reachable {
		if err := compiler.emitJump(bytecode.Jump, end, statement.Span()); err != nil {
			return err
		}
	}

	if err := compiler.mergeLabelDepth(handler, baseDepth+1, statement.Span()); err != nil {
		return err
	}
	if baseDepth+1 > compiler.maxStack {
		compiler.maxStack = baseDepth + 1
	}
	if err := compiler.markLabel(handler, statement.Span()); err != nil {
		return err
	}
	compiler.finallyDepth++
	err = compiler.compileStatements(statement.Finally)
	compiler.finallyDepth--
	if err != nil {
		return err
	}
	if compiler.reachable {
		if err := compiler.emitTerminator(bytecode.Reraise, 0, statement.Span()); err != nil {
			return err
		}
	}
	return compiler.markLabel(end, statement.Span())
}

// emitExceptionHandlerCleanup restores handled state, then clears and deletes
// the temporary exception binding while preserving any lower stack values.
func (compiler *compilerState) emitExceptionHandlerCleanup(cleanup exceptionHandlerCleanup) error {
	if err := compiler.emit(bytecode.LeaveExcept, 0, cleanup.span); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadConst,
		compiler.constantIndex(bytecode.None()),
		cleanup.span,
	); err != nil {
		return err
	}
	if err := compiler.emitNameStore(cleanup.name, cleanup.span); err != nil {
		return err
	}
	return compiler.emitNameDelete(cleanup.name, cleanup.span)
}

type controlCleanupState struct {
	cleanups []controlCleanup
	handlers []instructionExceptionHandler
}

// emitControlCleanupsFrom emits lexical cleanups from inner to outer and leaves
// their compiler state suspended so the caller can emit its transfer outside
// those protected regions. The returned snapshot must always be restored.
func (compiler *compilerState) emitControlCleanupsFrom(
	depth int,
	span lexer.Span,
) (controlCleanupState, error) {
	state := controlCleanupState{
		cleanups: slices.Clone(compiler.controlCleanups),
		handlers: slices.Clone(compiler.activeHandlers),
	}
	if depth < 0 || depth > len(compiler.controlCleanups) {
		return state, compiler.error(span, "control cleanup depth %d out of range", depth)
	}
	for len(compiler.controlCleanups) > depth && compiler.reachable {
		index := len(compiler.controlCleanups) - 1
		cleanup := compiler.controlCleanups[index]
		compiler.controlCleanups = compiler.controlCleanups[:index]
		if cleanup.handlerDepth < 0 || cleanup.handlerDepth > len(compiler.activeHandlers) {
			return state, compiler.error(span, "control cleanup handler depth out of range")
		}
		compiler.activeHandlers = compiler.activeHandlers[:cleanup.handlerDepth]

		var err error
		switch cleanup.kind {
		case exceptionHandlerControlCleanup:
			err = compiler.emitExceptionHandlerCleanup(cleanup.exception)
		case finallyControlCleanup:
			err = compiler.compileStatements(cleanup.finalBody)
		default:
			err = compiler.error(span, "unknown control cleanup kind %d", cleanup.kind)
		}
		if err != nil {
			return state, err
		}
	}
	return state, nil
}

func (compiler *compilerState) restoreControlCleanups(state controlCleanupState) {
	compiler.controlCleanups = state.cleanups
	compiler.activeHandlers = state.handlers
}

// finishedExceptionHandlers combines adjacent instructions protected by the
// same active handler into the immutable ranges stored on the code object.
func (compiler *compilerState) finishedExceptionHandlers() []bytecode.ExceptionHandler {
	var handlers []bytecode.ExceptionHandler
	for start := 0; start < len(compiler.exceptionHandlers); {
		current := compiler.exceptionHandlers[start]
		if current.target == nil {
			start++
			continue
		}
		end := start + 1
		for end < len(compiler.exceptionHandlers) {
			next := compiler.exceptionHandlers[end]
			if next.target != current.target || next.stackDepth != current.stackDepth {
				break
			}
			end++
		}
		handlers = append(handlers, bytecode.ExceptionHandler{
			Start:      uint32(start),
			End:        uint32(end),
			Target:     current.target.position,
			StackDepth: current.stackDepth,
		})
		start = end
	}
	return handlers
}
