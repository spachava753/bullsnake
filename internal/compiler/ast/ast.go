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

// Pattern is one structural pattern-matching form.
type Pattern interface {
	Node
	pattern()
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

// AugAssignStmt applies an operator and stores the result in one target.
type AugAssignStmt struct {
	Range  lexer.Span
	Target Expr
	Op     BinaryOperator
	Value  Expr
}

func (*AugAssignStmt) node() {}
func (*AugAssignStmt) stmt() {}

// Span returns the source range covered by the augmented assignment.
func (statement *AugAssignStmt) Span() lexer.Span { return statement.Range }

// AnnAssignStmt assigns an optional value to an annotated target.
type AnnAssignStmt struct {
	Range      lexer.Span
	Target     Expr
	Annotation Expr
	Value      Expr
	Simple     bool
}

func (*AnnAssignStmt) node() {}
func (*AnnAssignStmt) stmt() {}

// Span returns the source range covered by the annotated assignment.
func (statement *AnnAssignStmt) Span() lexer.Span { return statement.Range }

// ReturnStmt returns an optional value from a function.
type ReturnStmt struct {
	Range lexer.Span
	Value Expr
}

func (*ReturnStmt) node() {}
func (*ReturnStmt) stmt() {}

// Span returns the source range covered by the return statement.
func (statement *ReturnStmt) Span() lexer.Span { return statement.Range }

// RaiseStmt raises an optional exception with an optional explicit cause.
type RaiseStmt struct {
	Range     lexer.Span
	Exception Expr
	Cause     Expr
}

func (*RaiseStmt) node() {}
func (*RaiseStmt) stmt() {}

// Span returns the source range covered by the raise statement.
func (statement *RaiseStmt) Span() lexer.Span { return statement.Range }

// DeleteStmt deletes one or more targets.
type DeleteStmt struct {
	Range   lexer.Span
	Targets []Expr
}

func (*DeleteStmt) node() {}
func (*DeleteStmt) stmt() {}

// Span returns the source range covered by the delete statement.
func (statement *DeleteStmt) Span() lexer.Span { return statement.Range }

// AssertStmt checks a condition and carries an optional failure message.
type AssertStmt struct {
	Range     lexer.Span
	Condition Expr
	Message   Expr
}

func (*AssertStmt) node() {}
func (*AssertStmt) stmt() {}

// Span returns the source range covered by the assertion.
func (statement *AssertStmt) Span() lexer.Span { return statement.Range }

// BreakStmt exits the nearest loop.
type BreakStmt struct {
	Range lexer.Span
}

func (*BreakStmt) node() {}
func (*BreakStmt) stmt() {}

// Span returns the source range covered by the break statement.
func (statement *BreakStmt) Span() lexer.Span { return statement.Range }

// ContinueStmt starts the nearest loop's next iteration.
type ContinueStmt struct {
	Range lexer.Span
}

func (*ContinueStmt) node() {}
func (*ContinueStmt) stmt() {}

// Span returns the source range covered by the continue statement.
func (statement *ContinueStmt) Span() lexer.Span { return statement.Range }

// GlobalStmt declares names as module bindings in the current scope.
type GlobalStmt struct {
	Range lexer.Span
	Names []string
}

func (*GlobalStmt) node() {}
func (*GlobalStmt) stmt() {}

// Span returns the source range covered by the global statement.
func (statement *GlobalStmt) Span() lexer.Span { return statement.Range }

// NonlocalStmt declares names as enclosing-function bindings.
type NonlocalStmt struct {
	Range lexer.Span
	Names []string
}

func (*NonlocalStmt) node() {}
func (*NonlocalStmt) stmt() {}

// Span returns the source range covered by the nonlocal statement.
func (statement *NonlocalStmt) Span() lexer.Span { return statement.Range }

// ImportAlias is one imported dotted name and its optional local alias.
type ImportAlias struct {
	Range lexer.Span
	Name  string
	Alias string
}

// ImportStmt imports one or more modules.
type ImportStmt struct {
	Range lexer.Span
	Names []ImportAlias
}

func (*ImportStmt) node() {}
func (*ImportStmt) stmt() {}

// Span returns the source range covered by the import statement.
func (statement *ImportStmt) Span() lexer.Span { return statement.Range }

// FromImportStmt imports names from a relative or absolute module.
type FromImportStmt struct {
	Range    lexer.Span
	Module   string
	Names    []ImportAlias
	Level    int
	Wildcard bool
}

func (*FromImportStmt) node() {}
func (*FromImportStmt) stmt() {}

// Span returns the source range covered by the from-import statement.
func (statement *FromImportStmt) Span() lexer.Span { return statement.Range }

// PassStmt performs no operation.
type PassStmt struct {
	Range lexer.Span
}

func (*PassStmt) node() {}
func (*PassStmt) stmt() {}

// Span returns the source range covered by the pass statement.
func (statement *PassStmt) Span() lexer.Span { return statement.Range }

// IfStmt conditionally executes one statement list and an optional alternative.
type IfStmt struct {
	Range     lexer.Span
	Condition Expr
	Body      []Stmt
	Else      []Stmt
}

func (*IfStmt) node() {}
func (*IfStmt) stmt() {}

// Span returns the source range covered by the conditional statement.
func (statement *IfStmt) Span() lexer.Span { return statement.Range }

// WhileStmt repeatedly executes a body and has an optional normal-exit suite.
type WhileStmt struct {
	Range     lexer.Span
	Condition Expr
	Body      []Stmt
	Else      []Stmt
}

func (*WhileStmt) node() {}
func (*WhileStmt) stmt() {}

// Span returns the source range covered by the while statement.
func (statement *WhileStmt) Span() lexer.Span { return statement.Range }

// ForStmt iterates over a value and may be asynchronous or have an else suite.
type ForStmt struct {
	Range    lexer.Span
	Target   Expr
	Iterable Expr
	Body     []Stmt
	Else     []Stmt
	Async    bool
}

func (*ForStmt) node() {}
func (*ForStmt) stmt() {}

// Span returns the source range covered by the for statement.
func (statement *ForStmt) Span() lexer.Span { return statement.Range }

// WithItem is one context manager and optional assignment target.
type WithItem struct {
	Range   lexer.Span
	Context Expr
	Target  Expr
}

// WithStmt enters one or more synchronous or asynchronous context managers.
type WithStmt struct {
	Range lexer.Span
	Items []WithItem
	Body  []Stmt
	Async bool
}

func (*WithStmt) node() {}
func (*WithStmt) stmt() {}

// Span returns the source range covered by the with statement.
func (statement *WithStmt) Span() lexer.Span { return statement.Range }

// ExceptHandler is one except or except* clause.
type ExceptHandler struct {
	Range lexer.Span
	Type  Expr
	Name  string
	Body  []Stmt
	Star  bool
}

// TryStmt handles exceptions and optional normal and final suites.
type TryStmt struct {
	Range    lexer.Span
	Body     []Stmt
	Handlers []ExceptHandler
	Else     []Stmt
	Finally  []Stmt
}

func (*TryStmt) node() {}
func (*TryStmt) stmt() {}

// Span returns the source range covered by the try statement.
func (statement *TryStmt) Span() lexer.Span { return statement.Range }

// TypeParameterKind identifies a generic type parameter prefix.
type TypeParameterKind uint8

const (
	TypeVariable TypeParameterKind = iota
	TypeVariableTuple
	ParameterSpecification
)

var typeParameterKindNames = [...]string{"TypeVariable", "TypeVariableTuple", "ParameterSpecification"}

// String returns the AST spelling of a type parameter kind.
func (kind TypeParameterKind) String() string {
	if int(kind) >= len(typeParameterKindNames) {
		return fmt.Sprintf("TypeParameterKind(%d)", kind)
	}
	return typeParameterKindNames[kind]
}

// TypeParameter is one generic type variable, type-variable tuple, or
// parameter specification.
type TypeParameter struct {
	Range   lexer.Span
	Name    string
	Kind    TypeParameterKind
	Bound   Expr
	Default Expr
}

// Parameter is one named function or lambda parameter.
type Parameter struct {
	Range      lexer.Span
	Name       string
	Annotation Expr
	Default    Expr
}

// Parameters groups parameters by their Python calling convention.
type Parameters struct {
	PositionalOnly []Parameter
	Positional     []Parameter
	VarArg         *Parameter
	KeywordOnly    []Parameter
	KeywordVarArg  *Parameter
}

// FunctionDefStmt defines a synchronous or asynchronous function.
type FunctionDefStmt struct {
	Range          lexer.Span
	Name           string
	TypeParameters []TypeParameter
	Parameters     Parameters
	Returns        Expr
	Body           []Stmt
	Decorators     []Expr
	Async          bool
}

func (*FunctionDefStmt) node() {}
func (*FunctionDefStmt) stmt() {}

// Span returns the source range covered by the function definition.
func (statement *FunctionDefStmt) Span() lexer.Span { return statement.Range }

// ClassDefStmt defines a class with optional generic parameters and bases.
type ClassDefStmt struct {
	Range          lexer.Span
	Name           string
	TypeParameters []TypeParameter
	Bases          []Expr
	Keywords       []KeywordArgument
	Body           []Stmt
	Decorators     []Expr
}

func (*ClassDefStmt) node() {}
func (*ClassDefStmt) stmt() {}

// Span returns the source range covered by the class definition.
func (statement *ClassDefStmt) Span() lexer.Span { return statement.Range }

// TypeAliasStmt defines a possibly generic type alias.
type TypeAliasStmt struct {
	Range          lexer.Span
	Name           string
	TypeParameters []TypeParameter
	Value          Expr
}

func (*TypeAliasStmt) node() {}
func (*TypeAliasStmt) stmt() {}

// Span returns the source range covered by the type alias.
func (statement *TypeAliasStmt) Span() lexer.Span { return statement.Range }

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

// StringLiteral retains a plain string or bytes literal's source spelling.
type StringLiteral struct {
	Range lexer.Span
	Text  string
}

func (*StringLiteral) node() {}
func (*StringLiteral) expr() {}

// Span returns the source range covered by the literal.
func (literal *StringLiteral) Span() lexer.Span { return literal.Range }

// StringConcatExpr preserves adjacent plain, formatted, or template strings.
type StringConcatExpr struct {
	Range lexer.Span
	Parts []Expr
}

func (*StringConcatExpr) node() {}
func (*StringConcatExpr) expr() {}

// Span returns the source range covered by the adjacent strings.
func (expression *StringConcatExpr) Span() lexer.Span { return expression.Range }

// FormattedStringExpr is an f-string or template string split into parts.
type FormattedStringExpr struct {
	Range    lexer.Span
	Parts    []Expr
	Template bool
	Raw      bool
}

func (*FormattedStringExpr) node() {}
func (*FormattedStringExpr) expr() {}

// Span returns the source range covered by the formatted string.
func (expression *FormattedStringExpr) Span() lexer.Span { return expression.Range }

// FormattedValueExpr is one replacement field and optional format specifier.
type FormattedValueExpr struct {
	Range      lexer.Span
	Value      Expr
	Conversion string
	Format     []Expr // nil means no format colon; an empty non-nil slice means ":".
	Debug      bool
	DebugText  string
}

func (*FormattedValueExpr) node() {}
func (*FormattedValueExpr) expr() {}

// Span returns the source range covered by the replacement field.
func (expression *FormattedValueExpr) Span() lexer.Span { return expression.Range }

// BooleanLiteral is a True or False expression.
type BooleanLiteral struct {
	Range lexer.Span
	Value bool
}

func (*BooleanLiteral) node() {}
func (*BooleanLiteral) expr() {}

// Span returns the source range covered by the literal.
func (literal *BooleanLiteral) Span() lexer.Span { return literal.Range }

// NoneLiteral is the None expression.
type NoneLiteral struct {
	Range lexer.Span
}

func (*NoneLiteral) node() {}
func (*NoneLiteral) expr() {}

// Span returns the source range covered by the literal.
func (literal *NoneLiteral) Span() lexer.Span { return literal.Range }

// EllipsisLiteral is the ... expression.
type EllipsisLiteral struct {
	Range lexer.Span
}

func (*EllipsisLiteral) node() {}
func (*EllipsisLiteral) expr() {}

// Span returns the source range covered by the literal.
func (literal *EllipsisLiteral) Span() lexer.Span { return literal.Range }

// LambdaExpr defines an anonymous function.
type LambdaExpr struct {
	Range      lexer.Span
	Parameters Parameters
	Body       Expr
}

func (*LambdaExpr) node() {}
func (*LambdaExpr) expr() {}

// Span returns the source range covered by the lambda expression.
func (expression *LambdaExpr) Span() lexer.Span { return expression.Range }

// NamedExpr assigns and returns one value inside an expression.
type NamedExpr struct {
	Range  lexer.Span
	Target Expr
	Value  Expr
}

func (*NamedExpr) node() {}
func (*NamedExpr) expr() {}

// Span returns the source range covered by the assignment expression.
func (expression *NamedExpr) Span() lexer.Span { return expression.Range }

// ConditionalExpr chooses between two values.
type ConditionalExpr struct {
	Range     lexer.Span
	Condition Expr
	Then      Expr
	Else      Expr
}

func (*ConditionalExpr) node() {}
func (*ConditionalExpr) expr() {}

// Span returns the source range covered by the conditional expression.
func (expression *ConditionalExpr) Span() lexer.Span { return expression.Range }

// AwaitExpr suspends until its operand completes.
type AwaitExpr struct {
	Range lexer.Span
	Value Expr
}

func (*AwaitExpr) node() {}
func (*AwaitExpr) expr() {}

// Span returns the source range covered by the await expression.
func (expression *AwaitExpr) Span() lexer.Span { return expression.Range }

// YieldExpr yields an optional value or delegates with yield from.
type YieldExpr struct {
	Range lexer.Span
	Value Expr
	From  bool
}

func (*YieldExpr) node() {}
func (*YieldExpr) expr() {}

// Span returns the source range covered by the yield expression.
func (expression *YieldExpr) Span() lexer.Span { return expression.Range }

// UnaryOperator identifies an operation with one operand.
type UnaryOperator uint8

const (
	Positive UnaryOperator = iota
	Negative
	Invert
	Not
)

var unaryOperatorNames = [...]string{"Positive", "Negative", "Invert", "Not"}

// String returns the AST spelling of a unary operator.
func (operator UnaryOperator) String() string {
	if int(operator) >= len(unaryOperatorNames) {
		return fmt.Sprintf("UnaryOperator(%d)", operator)
	}
	return unaryOperatorNames[operator]
}

// UnaryExpr applies one unary operator to an expression.
type UnaryExpr struct {
	Range   lexer.Span
	Op      UnaryOperator
	Operand Expr
}

func (*UnaryExpr) node() {}
func (*UnaryExpr) expr() {}

// Span returns the source range covered by the unary expression.
func (expression *UnaryExpr) Span() lexer.Span { return expression.Range }

// BooleanOperator identifies a short-circuiting boolean operation.
type BooleanOperator uint8

const (
	And BooleanOperator = iota
	Or
)

var booleanOperatorNames = [...]string{"And", "Or"}

// String returns the AST spelling of a boolean operator.
func (operator BooleanOperator) String() string {
	if int(operator) >= len(booleanOperatorNames) {
		return fmt.Sprintf("BooleanOperator(%d)", operator)
	}
	return booleanOperatorNames[operator]
}

// BooleanExpr holds all values in one and/or chain.
type BooleanExpr struct {
	Range  lexer.Span
	Op     BooleanOperator
	Values []Expr
}

func (*BooleanExpr) node() {}
func (*BooleanExpr) expr() {}

// Span returns the source range covered by the boolean expression.
func (expression *BooleanExpr) Span() lexer.Span { return expression.Range }

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

// KeywordArgument is one named or dictionary-unpacked call argument.
// Name is empty for a **value argument.
type KeywordArgument struct {
	Range lexer.Span
	Name  string
	Value Expr
}

// CallExpr calls an expression with positional, starred, and keyword arguments.
type CallExpr struct {
	Range     lexer.Span
	Function  Expr
	Arguments []Expr
	Keywords  []KeywordArgument
}

func (*CallExpr) node() {}
func (*CallExpr) expr() {}

// Span returns the source range covered by the call.
func (expression *CallExpr) Span() lexer.Span { return expression.Range }

// AttributeExpr reads or writes one named attribute.
type AttributeExpr struct {
	Range   lexer.Span
	Value   Expr
	Name    string
	Context ExprContext
}

func (*AttributeExpr) node() {}
func (*AttributeExpr) expr() {}

// Span returns the source range covered by the attribute expression.
func (expression *AttributeExpr) Span() lexer.Span { return expression.Range }

// SubscriptExpr reads or writes one indexed value.
type SubscriptExpr struct {
	Range   lexer.Span
	Value   Expr
	Index   Expr
	Context ExprContext
}

func (*SubscriptExpr) node() {}
func (*SubscriptExpr) expr() {}

// Span returns the source range covered by the subscript expression.
func (expression *SubscriptExpr) Span() lexer.Span { return expression.Range }

// SliceExpr holds optional lower, upper, and step bounds.
type SliceExpr struct {
	Range lexer.Span
	Lower Expr
	Upper Expr
	Step  Expr
}

func (*SliceExpr) node() {}
func (*SliceExpr) expr() {}

// Span returns the source range covered by the slice expression.
func (expression *SliceExpr) Span() lexer.Span { return expression.Range }

// StarredExpr unpacks one value in a display, call, or assignment target.
type StarredExpr struct {
	Range   lexer.Span
	Value   Expr
	Context ExprContext
}

func (*StarredExpr) node() {}
func (*StarredExpr) expr() {}

// Span returns the source range covered by the starred expression.
func (expression *StarredExpr) Span() lexer.Span { return expression.Range }

// ListExpr is a list display or assignment target.
type ListExpr struct {
	Range    lexer.Span
	Elements []Expr
	Context  ExprContext
}

func (*ListExpr) node() {}
func (*ListExpr) expr() {}

// Span returns the source range covered by the list.
func (expression *ListExpr) Span() lexer.Span { return expression.Range }

// SetExpr is a set display.
type SetExpr struct {
	Range    lexer.Span
	Elements []Expr
}

func (*SetExpr) node() {}
func (*SetExpr) expr() {}

// Span returns the source range covered by the set.
func (expression *SetExpr) Span() lexer.Span { return expression.Range }

// DictExpr is a dictionary display. A nil key marks a **value entry.
type DictExpr struct {
	Range  lexer.Span
	Keys   []Expr
	Values []Expr
}

func (*DictExpr) node() {}
func (*DictExpr) expr() {}

// Span returns the source range covered by the dictionary.
func (expression *DictExpr) Span() lexer.Span { return expression.Range }

// Comprehension is one for clause and its following filters.
type Comprehension struct {
	Range      lexer.Span
	Target     Expr
	Iterable   Expr
	Conditions []Expr
	Async      bool
}

// ListComprehensionExpr constructs a list from comprehension clauses.
type ListComprehensionExpr struct {
	Range   lexer.Span
	Element Expr
	Clauses []Comprehension
}

func (*ListComprehensionExpr) node() {}
func (*ListComprehensionExpr) expr() {}

// Span returns the source range covered by the list comprehension.
func (expression *ListComprehensionExpr) Span() lexer.Span { return expression.Range }

// SetComprehensionExpr constructs a set from comprehension clauses.
type SetComprehensionExpr struct {
	Range   lexer.Span
	Element Expr
	Clauses []Comprehension
}

func (*SetComprehensionExpr) node() {}
func (*SetComprehensionExpr) expr() {}

// Span returns the source range covered by the set comprehension.
func (expression *SetComprehensionExpr) Span() lexer.Span { return expression.Range }

// DictComprehensionExpr constructs a dictionary from comprehension clauses.
type DictComprehensionExpr struct {
	Range   lexer.Span
	Key     Expr
	Value   Expr
	Clauses []Comprehension
}

func (*DictComprehensionExpr) node() {}
func (*DictComprehensionExpr) expr() {}

// Span returns the source range covered by the dictionary comprehension.
func (expression *DictComprehensionExpr) Span() lexer.Span { return expression.Range }

// GeneratorExpr lazily evaluates one element across comprehension clauses.
type GeneratorExpr struct {
	Range   lexer.Span
	Element Expr
	Clauses []Comprehension
}

func (*GeneratorExpr) node() {}
func (*GeneratorExpr) expr() {}

// Span returns the source range covered by the generator expression.
func (expression *GeneratorExpr) Span() lexer.Span { return expression.Range }

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
