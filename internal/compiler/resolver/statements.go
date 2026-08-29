package resolver

import (
	"strings"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
)

func (state *resolver) collectStatements(statements []compilerast.Stmt) error {
	for _, statement := range statements {
		if err := state.collectStatement(statement); err != nil {
			return err
		}
	}
	return nil
}

// collectStatement dispatches every AST statement and delegates scope-forming
// or context-sensitive forms to their focused collectors.
func (state *resolver) collectStatement(statement compilerast.Stmt) error {
	switch statement := statement.(type) {
	case *compilerast.ExprStmt:
		return state.collectExpr(statement.Value)
	case *compilerast.AssignStmt:
		for _, target := range statement.Targets {
			if err := state.collectExpr(target); err != nil {
				return err
			}
		}
		return state.collectExpr(statement.Value)
	case *compilerast.AugAssignStmt:
		if name, ok := statement.Target.(*compilerast.Name); ok {
			if _, err := state.bind(name.ID, Used|Assigned, name.Span()); err != nil {
				return err
			}
		} else if err := state.collectExpr(statement.Target); err != nil {
			return err
		}
		return state.collectExpr(statement.Value)
	case *compilerast.AnnAssignStmt:
		return state.collectAnnotatedAssignment(statement)
	case *compilerast.GlobalStmt:
		return state.collectGlobal(statement)
	case *compilerast.NonlocalStmt:
		return state.collectNonlocal(statement)
	case *compilerast.FunctionDefStmt:
		return state.collectFunction(statement)
	case *compilerast.ClassDefStmt:
		return state.collectClass(statement)
	case *compilerast.TypeAliasStmt:
		return state.collectTypeAlias(statement)
	case *compilerast.MatchStmt:
		return state.collectMatch(statement)
	case *compilerast.IfStmt:
		return state.collectIf(statement)
	case *compilerast.WhileStmt:
		return state.collectWhile(statement)
	case *compilerast.ForStmt:
		return state.collectFor(statement)
	case *compilerast.WithStmt:
		return state.collectWith(statement)
	case *compilerast.TryStmt:
		return state.collectTry(statement)
	case *compilerast.ReturnStmt:
		return state.collectReturn(statement)
	case *compilerast.RaiseStmt:
		if err := state.collectExpr(statement.Exception); err != nil {
			return err
		}
		return state.collectExpr(statement.Cause)
	case *compilerast.DeleteStmt:
		return state.collectExprs(statement.Targets)
	case *compilerast.AssertStmt:
		if err := state.collectExpr(statement.Condition); err != nil {
			return err
		}
		return state.collectExpr(statement.Message)
	case *compilerast.BreakStmt:
		return state.collectBreak(statement)
	case *compilerast.ContinueStmt:
		return state.collectContinue(statement)
	case *compilerast.ImportStmt:
		for _, imported := range statement.Names {
			name := imported.Alias
			if name == "" {
				name, _, _ = strings.Cut(imported.Name, ".")
			}
			if _, err := state.bind(name, Imported, imported.Range); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.FromImportStmt:
		if statement.Module == "__future__" && statement.Level == 0 && state.current.Kind != ModuleScope {
			return state.syntaxError(statement.Span(), "future imports must occur at the beginning of the file")
		}
		if statement.Wildcard {
			if state.current.Kind != ModuleScope {
				return state.syntaxError(statement.Span(), "import * only allowed at module level")
			}
			return nil
		}
		for _, imported := range statement.Names {
			name := imported.Alias
			if name == "" {
				name = imported.Name
			}
			if _, err := state.bind(name, Imported, imported.Range); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.PassStmt:
		return nil
	default:
		return unsupportedNode(statement)
	}
}
