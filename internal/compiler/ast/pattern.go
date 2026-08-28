package ast

import "github.com/spachava753/bullsnake/internal/compiler/lexer"

// MatchCase is one pattern, optional guard, and body suite.
type MatchCase struct {
	Range   lexer.Span
	Pattern Pattern
	Guard   Expr
	Body    []Stmt
}

// MatchStmt dispatches a subject to the first matching case.
type MatchStmt struct {
	Range   lexer.Span
	Subject Expr
	Cases   []MatchCase
}

func (*MatchStmt) node() {}
func (*MatchStmt) stmt() {}

// Span returns the source range covered by the match statement.
func (statement *MatchStmt) Span() lexer.Span { return statement.Range }

// ValuePattern compares the subject with a literal or dotted value.
type ValuePattern struct {
	Range lexer.Span
	Value Expr
}

func (*ValuePattern) node()    {}
func (*ValuePattern) pattern() {}

// Span returns the source range covered by the value pattern.
func (pattern *ValuePattern) Span() lexer.Span { return pattern.Range }

// CapturePattern binds the subject to a name.
type CapturePattern struct {
	Range lexer.Span
	Name  string
}

func (*CapturePattern) node()    {}
func (*CapturePattern) pattern() {}

// Span returns the source range covered by the capture pattern.
func (pattern *CapturePattern) Span() lexer.Span { return pattern.Range }

// WildcardPattern matches without binding.
type WildcardPattern struct {
	Range lexer.Span
}

func (*WildcardPattern) node()    {}
func (*WildcardPattern) pattern() {}

// Span returns the source range covered by the wildcard pattern.
func (pattern *WildcardPattern) Span() lexer.Span { return pattern.Range }

// StarPattern captures or discards the remainder of a sequence.
type StarPattern struct {
	Range lexer.Span
	Name  string
}

func (*StarPattern) node()    {}
func (*StarPattern) pattern() {}

// Span returns the source range covered by the star pattern.
func (pattern *StarPattern) Span() lexer.Span { return pattern.Range }

// SequencePattern matches a fixed sequence with optional starred remainder.
type SequencePattern struct {
	Range    lexer.Span
	Elements []Pattern
}

func (*SequencePattern) node()    {}
func (*SequencePattern) pattern() {}

// Span returns the source range covered by the sequence pattern.
func (pattern *SequencePattern) Span() lexer.Span { return pattern.Range }

// MappingPattern matches selected keys and an optional remaining mapping.
type MappingPattern struct {
	Range    lexer.Span
	Keys     []Expr
	Patterns []Pattern
	Rest     string
}

func (*MappingPattern) node()    {}
func (*MappingPattern) pattern() {}

// Span returns the source range covered by the mapping pattern.
func (pattern *MappingPattern) Span() lexer.Span { return pattern.Range }

// PatternKeyword is one named class-pattern field.
type PatternKeyword struct {
	Range   lexer.Span
	Name    string
	Pattern Pattern
}

// ClassPattern matches a class with positional and named subpatterns.
type ClassPattern struct {
	Range      lexer.Span
	Class      Expr
	Positional []Pattern
	Keywords   []PatternKeyword
}

func (*ClassPattern) node()    {}
func (*ClassPattern) pattern() {}

// Span returns the source range covered by the class pattern.
func (pattern *ClassPattern) Span() lexer.Span { return pattern.Range }

// OrPattern matches any one of its alternatives.
type OrPattern struct {
	Range    lexer.Span
	Patterns []Pattern
}

func (*OrPattern) node()    {}
func (*OrPattern) pattern() {}

// Span returns the source range covered by the OR pattern.
func (pattern *OrPattern) Span() lexer.Span { return pattern.Range }

// AsPattern binds the value matched by an optional inner pattern.
type AsPattern struct {
	Range   lexer.Span
	Pattern Pattern
	Name    string
}

func (*AsPattern) node()    {}
func (*AsPattern) pattern() {}

// Span returns the source range covered by the AS pattern.
func (pattern *AsPattern) Span() lexer.Span { return pattern.Range }
