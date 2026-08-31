package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
)

const (
	asyncIteratorName = "__aiter__"
	asyncNextName     = "__anext__"
)

// compileAsyncForStatement awaits each __anext__ result and catches
// StopAsyncIteration only around the next-item operation.
func (compiler *compilerState) compileAsyncForStatement(statement *compilerast.ForStmt) error {
	if compiler.scope.Kind != resolver.FunctionScope ||
		compiler.scope.Flags&resolver.Coroutine == 0 {
		return compiler.error(statement.Span(), "async for has no enclosing coroutine")
	}
	baseDepth := compiler.stackDepth
	start := compiler.newLabel()
	handler := compiler.newLabel()
	end := compiler.newLabel()
	normalExit := end
	if len(statement.Else) != 0 {
		normalExit = compiler.newLabel()
	}

	if err := compiler.compileExpr(statement.Iterable); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadSpecial,
		compiler.nameIndex(asyncIteratorName),
		statement.Iterable.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 0, statement.Iterable.Span()); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CheckAsyncIterator, 0, statement.Iterable.Span()); err != nil {
		return err
	}
	if err := compiler.markLabel(start, statement.Span()); err != nil {
		return err
	}

	handlerDepth := len(compiler.activeHandlers)
	compiler.activeHandlers = append(compiler.activeHandlers, instructionExceptionHandler{
		target:     handler,
		stackDepth: baseDepth + 1,
	})
	err := compiler.compileAsyncForNext(statement)
	compiler.activeHandlers = compiler.activeHandlers[:handlerDepth]
	if err != nil {
		return err
	}
	if err := compiler.compileStore(statement.Target); err != nil {
		return err
	}

	compiler.loops = append(compiler.loops, loopContext{
		continueLabel: start,
		breakLabel:    end,
		continueDepth: baseDepth + 1,
		breakDepth:    baseDepth,
		cleanupDepth:  len(compiler.controlCleanups),
	})
	if err := compiler.compileStatements(statement.Body); err != nil {
		return err
	}
	compiler.loops = compiler.loops[:len(compiler.loops)-1]
	if compiler.reachable {
		if err := compiler.emitJump(bytecode.Jump, start, statement.Span()); err != nil {
			return err
		}
	}

	if err := compiler.compileAsyncForExhaustion(
		handler,
		normalExit,
		baseDepth,
		statement.Span(),
	); err != nil {
		return err
	}
	if len(statement.Else) == 0 {
		return compiler.markLabel(end, statement.Span())
	}
	if err := compiler.markLabel(normalExit, statement.Span()); err != nil {
		return err
	}
	if err := compiler.compileStatements(statement.Else); err != nil {
		return err
	}
	return compiler.markLabel(end, statement.Span())
}

func (compiler *compilerState) compileAsyncForNext(statement *compilerast.ForStmt) error {
	if err := compiler.emit(bytecode.Copy, 1, statement.Iterable.Span()); err != nil {
		return err
	}
	if err := compiler.emit(
		bytecode.LoadSpecial,
		compiler.nameIndex(asyncNextName),
		statement.Iterable.Span(),
	); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Call, 0, statement.Iterable.Span()); err != nil {
		return err
	}
	return compiler.compileAwaitStackTop(statement.Span(), bytecode.AwaitAsyncNext)
}

// compileAsyncForExhaustion discards the iterator for StopAsyncIteration and
// reraises every other next-item failure.
func (compiler *compilerState) compileAsyncForExhaustion(
	handler *jumpLabel,
	normalExit *jumpLabel,
	baseDepth int,
	span lexer.Span,
) error {
	if err := compiler.mergeLabelDepth(handler, baseDepth+2, span); err != nil {
		return err
	}
	if baseDepth+2 > compiler.maxStack {
		compiler.maxStack = baseDepth + 2
	}
	if err := compiler.markLabel(handler, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.LoadStopAsyncIteration, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.CheckExceptionMatch, 0, span); err != nil {
		return err
	}
	mismatch := compiler.newLabel()
	if err := compiler.emitJump(bytecode.PopJumpIfFalse, mismatch, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
		return err
	}
	if err := compiler.emitJump(bytecode.Jump, normalExit, span); err != nil {
		return err
	}

	if err := compiler.markLabel(mismatch, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.Swap, 2, span); err != nil {
		return err
	}
	if err := compiler.emit(bytecode.PopTop, 0, span); err != nil {
		return err
	}
	return compiler.emitTerminator(bytecode.Reraise, 0, span)
}
