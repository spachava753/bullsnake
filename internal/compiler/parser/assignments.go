package parser

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// parseAssignmentOrExpression parses the shared expression prefix once, then
// selects expression, chained, annotated, or augmented assignment syntax.
func (parser *parserState) parseAssignmentOrExpression() (compilerast.Stmt, error) {
	left, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}

	if _, annotated, err := parser.take(lexer.Colon); err != nil {
		return nil, err
	} else if annotated {
		return parser.finishAnnotatedAssignment(left)
	}
	if operator, matched, err := parser.takeAugmentedOperator(); err != nil {
		return nil, err
	} else if matched {
		return parser.finishAugmentedAssignment(left, operator)
	}

	current := left
	var targets []compilerast.Expr
	for {
		_, matched, err := parser.take(lexer.Equal)
		if err != nil {
			return nil, err
		}
		if !matched {
			break
		}
		if starred, ok := current.(*compilerast.StarredExpr); ok {
			return nil, parser.errorAt(starred.Span(), "starred target must be in a list or tuple", false)
		}
		if err := parser.setStoreContext(current); err != nil {
			return nil, err
		}
		targets = append(targets, current)
		current, err = parser.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	if len(targets) != 0 {
		return &compilerast.AssignStmt{
			Range:   joinSpans(targets[0].Span(), current.Span()),
			Targets: targets,
			Value:   current,
		}, nil
	}
	if starred, ok := left.(*compilerast.StarredExpr); ok {
		return nil, parser.errorAt(starred.Span(), "starred expression must be in an expression list", false)
	}
	return &compilerast.ExprStmt{Range: left.Span(), Value: left}, nil
}

// finishAnnotatedAssignment validates the restricted target, parses the
// annotation, and accepts an optional assigned value.
func (parser *parserState) finishAnnotatedAssignment(target compilerast.Expr) (compilerast.Stmt, error) {
	simple := false
	switch target.(type) {
	case *compilerast.Name:
		simple = true
	case *compilerast.AttributeExpr, *compilerast.SubscriptExpr:
	default:
		return nil, parser.errorAt(target.Span(), "invalid annotated assignment target", false)
	}
	if err := parser.setStoreContext(target); err != nil {
		return nil, err
	}
	annotation, err := parser.parseConditionalExpression()
	if err != nil {
		return nil, err
	}
	end := annotation.Span()
	var value compilerast.Expr
	if _, matched, err := parser.take(lexer.Equal); err != nil {
		return nil, err
	} else if matched {
		value, err = parser.parseExpression()
		if err != nil {
			return nil, err
		}
		end = value.Span()
	}
	return &compilerast.AnnAssignStmt{
		Range:      joinSpans(target.Span(), end),
		Target:     target,
		Annotation: annotation,
		Value:      value,
		Simple:     simple,
	}, nil
}

func (parser *parserState) finishAugmentedAssignment(
	target compilerast.Expr,
	operator compilerast.BinaryOperator,
) (compilerast.Stmt, error) {
	switch target.(type) {
	case *compilerast.Name, *compilerast.AttributeExpr, *compilerast.SubscriptExpr:
	default:
		return nil, parser.errorAt(target.Span(), "invalid augmented assignment target", false)
	}
	if err := parser.setStoreContext(target); err != nil {
		return nil, err
	}
	value, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	return &compilerast.AugAssignStmt{
		Range:  joinSpans(target.Span(), value.Span()),
		Target: target,
		Op:     operator,
		Value:  value,
	}, nil
}

func (parser *parserState) takeAugmentedOperator() (compilerast.BinaryOperator, bool, error) {
	token, err := parser.peek(0)
	if err != nil {
		return 0, false, err
	}
	operator, matched := augmentedOperators[token.Kind]
	if !matched {
		return 0, false, nil
	}
	_, err = parser.advance()
	return operator, true, err
}

// setStoreContext marks valid assignment targets as Store and recursively
// updates sequence elements.
func (parser *parserState) setStoreContext(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		expression.Context = compilerast.Store
		return nil
	case *compilerast.TupleExpr:
		expression.Context = compilerast.Store
		return parser.setSequenceStoreContext(expression.Elements)
	case *compilerast.ListExpr:
		expression.Context = compilerast.Store
		return parser.setSequenceStoreContext(expression.Elements)
	case *compilerast.StarredExpr:
		expression.Context = compilerast.Store
		return parser.setStoreContext(expression.Value)
	case *compilerast.AttributeExpr:
		expression.Context = compilerast.Store
		return nil
	case *compilerast.SubscriptExpr:
		expression.Context = compilerast.Store
		return nil
	default:
		return parser.errorAt(expression.Span(), "invalid assignment target", false)
	}
}

func (parser *parserState) setSequenceStoreContext(elements []compilerast.Expr) error {
	starred := 0
	for _, element := range elements {
		if _, ok := element.(*compilerast.StarredExpr); ok {
			starred++
			if starred > 1 {
				return parser.errorAt(element.Span(), "multiple starred expressions in assignment", false)
			}
		}
		if err := parser.setStoreContext(element); err != nil {
			return err
		}
	}
	return nil
}

// setDeleteContext marks valid delete targets and recursively updates tuple or
// list elements while rejecting starred and value-only forms.
func (parser *parserState) setDeleteContext(expression compilerast.Expr) error {
	switch expression := expression.(type) {
	case *compilerast.Name:
		expression.Context = compilerast.Delete
		return nil
	case *compilerast.TupleExpr:
		expression.Context = compilerast.Delete
		for _, element := range expression.Elements {
			if err := parser.setDeleteContext(element); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.ListExpr:
		expression.Context = compilerast.Delete
		for _, element := range expression.Elements {
			if err := parser.setDeleteContext(element); err != nil {
				return err
			}
		}
		return nil
	case *compilerast.AttributeExpr:
		expression.Context = compilerast.Delete
		return nil
	case *compilerast.SubscriptExpr:
		expression.Context = compilerast.Delete
		return nil
	default:
		return parser.errorAt(expression.Span(), "invalid delete target", false)
	}
}

var augmentedOperators = map[lexer.Kind]compilerast.BinaryOperator{
	lexer.PlusEqual:        compilerast.Add,
	lexer.MinusEqual:       compilerast.Subtract,
	lexer.StarEqual:        compilerast.Multiply,
	lexer.AtEqual:          compilerast.MatrixMultiply,
	lexer.SlashEqual:       compilerast.Divide,
	lexer.DoubleSlashEqual: compilerast.FloorDivide,
	lexer.PercentEqual:     compilerast.Modulo,
	lexer.DoubleStarEqual:  compilerast.Power,
	lexer.LeftShiftEqual:   compilerast.LeftShift,
	lexer.RightShiftEqual:  compilerast.RightShift,
	lexer.VBarEqual:        compilerast.BitOr,
	lexer.CircumflexEqual:  compilerast.BitXor,
	lexer.AmpersandEqual:   compilerast.BitAnd,
}
