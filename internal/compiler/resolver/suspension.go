package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

// collectYield checks scope placement, marks the containing function as a
// generator, and visits the yielded value.
func (state *resolver) collectYield(expression *compilerast.YieldExpr) error {
	if state.current.Kind == AnnotationScope {
		return state.syntaxError(expression.Span(), "yield expression cannot be used within an annotation")
	}
	if state.current.Kind != FunctionScope {
		return state.syntaxError(expression.Span(), "yield outside function")
	}
	if isComprehension(state.current) {
		return state.syntaxError(expression.Span(), "yield inside %s", comprehensionDisplayName(state.current))
	}
	if expression.From && state.current.Flags&AsyncFunction != 0 {
		return state.syntaxError(expression.Span(), "yield from inside async function")
	}
	state.current.Flags |= Generator
	return state.collectExpr(expression.Value)
}

func (state *resolver) collectAwait(expression *compilerast.AwaitExpr) error {
	if state.current.Kind == AnnotationScope {
		return state.syntaxError(expression.Span(), "await expression cannot be used within an annotation")
	}
	if state.current.Kind != FunctionScope {
		return state.syntaxError(expression.Span(), "await outside function")
	}
	if state.current.Flags&AsyncFunction == 0 && !isComprehension(state.current) {
		return state.syntaxError(expression.Span(), "await outside async function")
	}
	state.current.Flags |= Coroutine
	return state.collectExpr(expression.Value)
}

func comprehensionDisplayName(scope *Scope) string {
	switch {
	case scope.Flags&ListComprehension != 0:
		return "list comprehension"
	case scope.Flags&SetComprehension != 0:
		return "set comprehension"
	case scope.Flags&DictComprehension != 0:
		return "dict comprehension"
	default:
		return "generator expression"
	}
}
