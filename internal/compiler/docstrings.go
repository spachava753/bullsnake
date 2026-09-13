package compiler

import (
	"strings"

	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
)

// compileFunctionBody moves only a leading plain text literal into immutable
// metadata; bytes, f-strings, templates, and later strings remain statements.
func (compiler *compilerState) compileFunctionBody(body []compilerast.Stmt) error {
	if len(body) != 0 {
		if statement, ok := body[0].(*compilerast.ExprStmt); ok {
			text, found, err := compiler.functionDocstring(statement.Value)
			if err != nil {
				return err
			}
			if found {
				compiler.docstring = &text
				body = body[1:]
			}
		}
	}
	return compiler.compileStatements(body)
}

// functionDocstring decodes adjacent plain text literals using the ordinary
// string decoder, preserving whitespace, escapes, and an explicitly empty doc.
func (compiler *compilerState) functionDocstring(expression compilerast.Expr) (string, bool, error) {
	parts := []compilerast.Expr{expression}
	if concat, ok := expression.(*compilerast.StringConcatExpr); ok {
		parts = concat.Parts
	}
	var text strings.Builder
	for _, part := range parts {
		literal, ok := part.(*compilerast.StringLiteral)
		if !ok {
			return "", false, nil
		}
		constant, err := parseStringLiteral(literal.Text)
		if err != nil {
			return "", false, compiler.error(literal.Span(), "%v", err)
		}
		if constant.Kind != bytecode.StringConstant {
			return "", false, nil
		}
		text.WriteString(constant.Text)
	}
	return text.String(), true, nil
}
