package resolver

import compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"

var knownFutureFeatures = map[string]Features{
	"nested_scopes":    0,
	"generators":       0,
	"division":         0,
	"absolute_import":  0,
	"with_statement":   0,
	"print_function":   0,
	"unicode_literals": 0,
	"barry_as_FLUFL":   0,
	"generator_stop":   0,
	"annotations":      FutureAnnotations,
}

// scanFutureFeatures reads allowed leading future imports before collection so
// annotation handling sees the module's final feature set.
func (state *resolver) scanFutureFeatures(module *compilerast.Module) error {
	atBeginning := true
	for index, statement := range module.Body {
		if index == 0 && isDocstringStatement(statement) {
			continue
		}
		importStatement, futureImport := statement.(*compilerast.FromImportStmt)
		if !futureImport || importStatement.Level != 0 || importStatement.Module != "__future__" {
			atBeginning = false
			continue
		}
		if !atBeginning {
			return state.syntaxError(statement.Span(), "future imports must occur at the beginning of the file")
		}
		for _, imported := range importStatement.Names {
			feature, known := knownFutureFeatures[imported.Name]
			if !known {
				return state.syntaxError(
					imported.Range,
					"future feature %s is not defined",
					imported.Name,
				)
			}
			state.table.Features |= feature
		}
	}
	return nil
}

func isDocstringStatement(statement compilerast.Stmt) bool {
	expressionStatement, ok := statement.(*compilerast.ExprStmt)
	if !ok {
		return false
	}
	switch expressionStatement.Value.(type) {
	case *compilerast.StringLiteral, *compilerast.StringConcatExpr:
		return true
	default:
		return false
	}
}
