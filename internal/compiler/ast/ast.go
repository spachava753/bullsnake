// Package ast defines Bullsnake's internal compiler syntax tree.
package ast

import (
	"fmt"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

// Node is a source-located AST node.
type Node interface {
	Span() lexer.Span
	node()
}

// Stmt is a Python statement.
type Stmt interface {
	Node
	stmt()
}

// Expr is a Python expression.
type Expr interface {
	Node
	expr()
}

// Module is the root produced for one source input.
type Module struct {
	Range lexer.Span
	Body  []Stmt
}

func (*Module) node() {}

// Span returns the source range covered by the module.
func (module *Module) Span() lexer.Span { return module.Range }

// ExprStmt evaluates an expression as a statement.
type ExprStmt struct {
	Range lexer.Span
	Value Expr
}

func (*ExprStmt) node() {}
func (*ExprStmt) stmt() {}

// Span returns the source range covered by the statement.
func (statement *ExprStmt) Span() lexer.Span { return statement.Range }

// AssignStmt assigns one value to one or more targets.
type AssignStmt struct {
	Range   lexer.Span
	Targets []Expr
	Value   Expr
}

func (*AssignStmt) node() {}
func (*AssignStmt) stmt() {}

// Span returns the source range covered by the assignment.
func (statement *AssignStmt) Span() lexer.Span { return statement.Range }

// ExprContext identifies how an expression is used.
type ExprContext uint8

const (
	Load ExprContext = iota
	Store
	Delete
)

// String returns the AST spelling of an expression context.
func (context ExprContext) String() string {
	switch context {
	case Load:
		return "Load"
	case Store:
		return "Store"
	case Delete:
		return "Delete"
	default:
		return fmt.Sprintf("ExprContext(%d)", context)
	}
}

// Name is an identifier expression.
type Name struct {
	Range   lexer.Span
	ID      string
	Context ExprContext
}

func (*Name) node() {}
func (*Name) expr() {}

// Span returns the source range covered by the identifier.
func (name *Name) Span() lexer.Span { return name.Range }

// NumberLiteral retains a numeric literal's exact source spelling.
type NumberLiteral struct {
	Range lexer.Span
	Text  string
}

func (*NumberLiteral) node() {}
func (*NumberLiteral) expr() {}

// Span returns the source range covered by the literal.
func (literal *NumberLiteral) Span() lexer.Span { return literal.Range }

// BinaryOperator identifies an ordinary binary operation.
type BinaryOperator uint8

const (
	Add BinaryOperator = iota
	Subtract
	Multiply
	MatrixMultiply
	Divide
	FloorDivide
	Modulo
	Power
	LeftShift
	RightShift
	BitOr
	BitXor
	BitAnd
)

var binaryOperatorNames = [...]string{
	"Add",
	"Subtract",
	"Multiply",
	"MatrixMultiply",
	"Divide",
	"FloorDivide",
	"Modulo",
	"Power",
	"LeftShift",
	"RightShift",
	"BitOr",
	"BitXor",
	"BitAnd",
}

// String returns the AST spelling of a binary operator.
func (operator BinaryOperator) String() string {
	if int(operator) >= len(binaryOperatorNames) {
		return fmt.Sprintf("BinaryOperator(%d)", operator)
	}
	return binaryOperatorNames[operator]
}

// BinaryExpr applies a binary operator to two expressions.
type BinaryExpr struct {
	Range lexer.Span
	Left  Expr
	Op    BinaryOperator
	Right Expr
}

func (*BinaryExpr) node() {}
func (*BinaryExpr) expr() {}

// Span returns the source range covered by the binary expression.
func (expression *BinaryExpr) Span() lexer.Span { return expression.Range }

// ComparisonOperator identifies one operation in a comparison chain.
type ComparisonOperator uint8

const (
	Equal ComparisonOperator = iota
	NotEqual
	Less
	LessEqual
	Greater
	GreaterEqual
	In
	NotIn
	Is
	IsNot
)

var comparisonOperatorNames = [...]string{
	"Equal",
	"NotEqual",
	"Less",
	"LessEqual",
	"Greater",
	"GreaterEqual",
	"In",
	"NotIn",
	"Is",
	"IsNot",
}

// String returns the AST spelling of a comparison operator.
func (operator ComparisonOperator) String() string {
	if int(operator) >= len(comparisonOperatorNames) {
		return fmt.Sprintf("ComparisonOperator(%d)", operator)
	}
	return comparisonOperatorNames[operator]
}

// CompareExpr holds all operations and right operands in a comparison chain.
type CompareExpr struct {
	Range       lexer.Span
	Left        Expr
	Operators   []ComparisonOperator
	Comparators []Expr
}

func (*CompareExpr) node() {}
func (*CompareExpr) expr() {}

// Span returns the source range covered by the comparison.
func (expression *CompareExpr) Span() lexer.Span { return expression.Range }

// CallExpr calls an expression with positional arguments.
// Keyword and unpacking argument forms will extend this node with the grammar.
type CallExpr struct {
	Range     lexer.Span
	Function  Expr
	Arguments []Expr
}

func (*CallExpr) node() {}
func (*CallExpr) expr() {}

// Span returns the source range covered by the call.
func (expression *CallExpr) Span() lexer.Span { return expression.Range }

// TupleExpr is a tuple display or an unparenthesized tuple expression.
type TupleExpr struct {
	Range    lexer.Span
	Elements []Expr
	Context  ExprContext
}

func (*TupleExpr) node() {}
func (*TupleExpr) expr() {}

// Span returns the source range covered by the tuple.
func (expression *TupleExpr) Span() lexer.Span { return expression.Range }
