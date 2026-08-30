package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

type instructionExceptionHandler struct {
	target     *jumpLabel
	stackDepth int
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
		return compiler.error(statement.Span(), "try/finally is not compiled")
	}
	if len(statement.Handlers) == 0 {
		return compiler.error(statement.Span(), "try statement has no exception handlers")
	}
	for index, handler := range statement.Handlers {
		if handler.Star {
			return compiler.error(handler.Range, "exception-group handlers are not compiled")
		}
		if handler.Name != "" {
			return compiler.error(handler.Range, "exception handler bindings are not compiled")
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
		if err := compiler.emit(bytecode.PopTop, 0, handler.Range); err != nil {
			return err
		}
		if err := compiler.compileStatements(handler.Body); err != nil {
			return err
		}
		if next == nil {
			return compiler.markLabel(end, statement.Span())
		}
		if compiler.reachable {
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
