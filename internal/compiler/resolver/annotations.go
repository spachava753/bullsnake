package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

// collectAnnotatedAssignment records the target, reuses its owner's annotation
// scope, and evaluates an optional value in the surrounding scope.
func (state *resolver) collectAnnotatedAssignment(statement *compilerast.AnnAssignStmt) error {
	state.current.Flags |= UsesAnnotations
	if name, ok := statement.Target.(*compilerast.Name); ok && statement.Simple {
		mangled := manglePrivate(state.current.PrivateName, name.ID)
		if state.current.Kind != ModuleScope {
			if symbol := state.current.Symbols[mangled]; symbol != nil {
				switch {
				case symbol.Flags&GlobalDeclaration != 0:
					return state.syntaxError(statement.Span(), "annotated name %q cannot be global", mangled)
				case symbol.Flags&NonlocalDeclaration != 0:
					return state.syntaxError(statement.Span(), "annotated name %q cannot be nonlocal", mangled)
				}
			}
		}
		if _, err := state.bind(name.ID, Assigned|Annotated, name.Span()); err != nil {
			return err
		}
	} else if err := state.collectExpr(statement.Target); err != nil {
		return err
	}

	localAnnotation := state.current.Kind == FunctionScope
	if state.table.Features&FutureAnnotations == 0 || localAnnotation {
		annotationScope := state.annotations[state.current]
		if annotationScope == nil {
			annotationFlags := UsesAnnotations
			if localAnnotation {
				annotationFlags |= UnevaluatedAnnotations
			} else if state.current.Kind == ClassScope {
				annotationFlags |= CanSeeClassScope
				state.current.Flags |= NeedsClassDict
			}
			annotationScope = state.childScope(
				statement, Annotations, 0, AnnotationScope, "__annotate__",
				state.current.PrivateName, annotationFlags,
			)
			state.annotations[state.current] = annotationScope
		} else {
			state.table.scopes[scopeKey{node: statement, purpose: Annotations}] = annotationScope
		}
		if err := state.inScope(annotationScope, func() error {
			return state.collectExpr(statement.Annotation)
		}); err != nil {
			return err
		}
	}
	return state.collectExpr(statement.Value)
}
