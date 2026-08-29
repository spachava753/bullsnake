package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

func (state *resolver) collectIf(statement *compilerast.IfStmt) error {
	if err := state.collectExpr(statement.Condition); err != nil {
		return err
	}
	if err := state.collectStatements(statement.Body); err != nil {
		return err
	}
	return state.collectStatements(statement.Else)
}

func (state *resolver) collectWhile(statement *compilerast.WhileStmt) error {
	if err := state.collectExpr(statement.Condition); err != nil {
		return err
	}
	if err := state.collectLoopBody(statement.Body); err != nil {
		return err
	}
	return state.collectStatements(statement.Else)
}

// collectFor checks asynchronous placement, visits target and iterable, and
// exposes loop control only while walking the loop body.
func (state *resolver) collectFor(statement *compilerast.ForStmt) error {
	if statement.Async && !state.inAsyncFunction() {
		return state.syntaxError(statement.Span(), "async for outside async function")
	}
	if err := state.collectExpr(statement.Target); err != nil {
		return err
	}
	if err := state.collectExpr(statement.Iterable); err != nil {
		return err
	}
	if err := state.collectLoopBody(statement.Body); err != nil {
		return err
	}
	return state.collectStatements(statement.Else)
}

func (state *resolver) collectLoopBody(body []compilerast.Stmt) error {
	state.control.loopDepth++
	err := state.collectStatements(body)
	state.control.loopDepth--
	return err
}

// collectWith checks asynchronous placement and visits each manager before its
// optional binding target and the shared body.
func (state *resolver) collectWith(statement *compilerast.WithStmt) error {
	if statement.Async && !state.inAsyncFunction() {
		return state.syntaxError(statement.Span(), "async with outside async function")
	}
	for _, item := range statement.Items {
		if err := state.collectExpr(item.Context); err != nil {
			return err
		}
		if err := state.collectExpr(item.Target); err != nil {
			return err
		}
	}
	return state.collectStatements(statement.Body)
}

// collectTry visits handlers in source order and limits return and loop control
// while walking an exception-group handler body.
func (state *resolver) collectTry(statement *compilerast.TryStmt) error {
	if err := state.collectStatements(statement.Body); err != nil {
		return err
	}
	for _, handler := range statement.Handlers {
		if err := state.collectExpr(handler.Type); err != nil {
			return err
		}
		if handler.Name != "" {
			if _, err := state.bind(handler.Name, Assigned, handler.Range); err != nil {
				return err
			}
		}
		if handler.Star {
			state.control.exceptStarDepth++
		}
		err := state.collectStatements(handler.Body)
		if handler.Star {
			state.control.exceptStarDepth--
		}
		if err != nil {
			return err
		}
	}
	if err := state.collectStatements(statement.Else); err != nil {
		return err
	}
	return state.collectStatements(statement.Finally)
}

func (state *resolver) collectReturn(statement *compilerast.ReturnStmt) error {
	if state.current.Kind != FunctionScope || isComprehension(state.current) {
		return state.syntaxError(statement.Span(), "return outside function")
	}
	if state.control.exceptStarDepth != 0 {
		return state.syntaxError(statement.Span(), "return cannot appear in an except* block")
	}
	if statement.Value != nil {
		state.current.Flags |= ReturnsValue
		state.current.returnValueSpan = statement.Span()
	}
	return state.collectExpr(statement.Value)
}

func (state *resolver) collectBreak(statement *compilerast.BreakStmt) error {
	if state.control.exceptStarDepth != 0 {
		return state.syntaxError(statement.Span(), "break cannot appear in an except* block")
	}
	if state.control.loopDepth == 0 {
		return state.syntaxError(statement.Span(), "break outside loop")
	}
	return nil
}

func (state *resolver) collectContinue(statement *compilerast.ContinueStmt) error {
	if state.control.exceptStarDepth != 0 {
		return state.syntaxError(statement.Span(), "continue cannot appear in an except* block")
	}
	if state.control.loopDepth == 0 {
		return state.syntaxError(statement.Span(), "continue not properly in loop")
	}
	return nil
}

func (state *resolver) inAsyncFunction() bool {
	return state.current.Kind == FunctionScope && state.current.Flags&AsyncFunction != 0
}
