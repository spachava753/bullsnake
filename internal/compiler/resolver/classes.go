package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

// collectClass resolves decorators outside the class and places generic bases
// inside the type-parameter scope before entering the class body.
func (state *resolver) collectClass(statement *compilerast.ClassDefStmt) error {
	if _, err := state.bind(statement.Name, Assigned, statement.Span()); err != nil {
		return err
	}
	for _, decorator := range statement.Decorators {
		if err := state.collectExpr(decorator); err != nil {
			return err
		}
	}
	if len(statement.TypeParameters) == 0 {
		if err := state.collectClassBases(statement); err != nil {
			return err
		}
		return state.collectClassBody(statement)
	}

	canSeeClass := state.current.Kind == ClassScope || state.current.Flags&CanSeeClassScope != 0
	return state.inTypeParameters(
		statement, statement.Name, statement.TypeParameters,
		statement.Name, canSeeClass,
		func() error {
			if err := state.collectClassBases(statement); err != nil {
				return err
			}
			return state.collectClassBody(statement)
		},
	)
}

func (state *resolver) collectClassBases(statement *compilerast.ClassDefStmt) error {
	for _, base := range statement.Bases {
		if err := state.collectExpr(base); err != nil {
			return err
		}
	}
	for _, keyword := range statement.Keywords {
		if err := state.collectExpr(keyword.Value); err != nil {
			return err
		}
	}
	return nil
}

func (state *resolver) collectClassBody(statement *compilerast.ClassDefStmt) error {
	classScope := state.childScope(
		statement, DefinitionBody, 0, ClassScope, statement.Name, statement.Name, 0,
	)
	return state.inScope(classScope, func() error {
		return state.collectStatements(statement.Body)
	})
}
