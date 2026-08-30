package compiler

import (
	compilerast "github.com/spachava753/bullsnake/internal/compiler/ast"
	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

type matchCapture struct {
	name string
	span lexer.Span
}

// compileMatchStatement evaluates one subject and dispatches to the first
// basic pattern whose optional guard succeeds.
func (compiler *compilerState) compileMatchStatement(
	statement *compilerast.MatchStmt,
) error {
	if len(statement.Cases) == 0 {
		return compiler.error(statement.Span(), "match statement has no cases")
	}
	if err := compiler.compileExpr(statement.Subject); err != nil {
		return err
	}

	end := compiler.newLabel()
	for _, matchCase := range statement.Cases {
		matched := compiler.newLabel()
		failed := compiler.newLabel()
		captures, err := compiler.basicPatternCaptures(matchCase.Pattern)
		if err != nil {
			return err
		}
		if err := compiler.compileBasicPattern(matchCase.Pattern, matched, failed); err != nil {
			return err
		}
		if err := compiler.markLabel(matched, matchCase.Pattern.Span()); err != nil {
			return err
		}
		for _, capture := range captures {
			if err := compiler.emit(bytecode.Copy, 1, capture.span); err != nil {
				return err
			}
			if err := compiler.emitNameStore(capture.name, capture.span); err != nil {
				return err
			}
		}
		if matchCase.Guard != nil {
			if err := compiler.compileExpr(matchCase.Guard); err != nil {
				return err
			}
			if err := compiler.emitJump(
				bytecode.PopJumpIfFalse,
				failed,
				matchCase.Guard.Span(),
			); err != nil {
				return err
			}
		}
		if err := compiler.emit(bytecode.PopTop, 0, matchCase.Pattern.Span()); err != nil {
			return err
		}
		if err := compiler.compileStatements(matchCase.Body); err != nil {
			return err
		}
		if compiler.reachable {
			if err := compiler.emitJump(bytecode.Jump, end, matchCase.Range); err != nil {
				return err
			}
		}
		if err := compiler.markLabel(failed, matchCase.Pattern.Span()); err != nil {
			return err
		}
	}
	if compiler.reachable {
		if err := compiler.emit(bytecode.PopTop, 0, statement.Span()); err != nil {
			return err
		}
	}
	return compiler.markLabel(end, statement.Span())
}

// compileBasicPattern tests one pattern while retaining the match subject on
// the operand stack along both success and failure paths.
func (compiler *compilerState) compileBasicPattern(
	pattern compilerast.Pattern,
	matched *jumpLabel,
	failed *jumpLabel,
) error {
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern:
		if err := compiler.emit(bytecode.Copy, 1, pattern.Span()); err != nil {
			return err
		}
		if err := compiler.compileExpr(pattern.Value); err != nil {
			return err
		}
		comparison := uint32(bytecode.CompareEqual)
		switch pattern.Value.(type) {
		case *compilerast.NoneLiteral, *compilerast.BooleanLiteral:
			comparison = bytecode.CompareIs
		}
		if err := compiler.emit(bytecode.CompareOp, comparison, pattern.Span()); err != nil {
			return err
		}
		if err := compiler.emitJump(bytecode.PopJumpIfTrue, matched, pattern.Span()); err != nil {
			return err
		}
		return compiler.emitJump(bytecode.Jump, failed, pattern.Span())
	case *compilerast.CapturePattern, *compilerast.WildcardPattern:
		return compiler.emitJump(bytecode.Jump, matched, pattern.Span())
	case *compilerast.AsPattern:
		if pattern.Pattern == nil {
			return compiler.emitJump(bytecode.Jump, matched, pattern.Span())
		}
		return compiler.compileBasicPattern(pattern.Pattern, matched, failed)
	case *compilerast.OrPattern:
		if len(pattern.Patterns) == 0 {
			return compiler.error(pattern.Span(), "OR pattern has no alternatives")
		}
		for _, alternative := range pattern.Patterns[:len(pattern.Patterns)-1] {
			next := compiler.newLabel()
			if err := compiler.compileBasicPattern(alternative, matched, next); err != nil {
				return err
			}
			if err := compiler.markLabel(next, alternative.Span()); err != nil {
				return err
			}
		}
		return compiler.compileBasicPattern(
			pattern.Patterns[len(pattern.Patterns)-1],
			matched,
			failed,
		)
	default:
		return compiler.unsupported(pattern)
	}
}

// basicPatternCaptures returns the names committed after a successful basic
// pattern; OR alternatives already have equal capture sets from resolution.
func (compiler *compilerState) basicPatternCaptures(
	pattern compilerast.Pattern,
) ([]matchCapture, error) {
	switch pattern := pattern.(type) {
	case *compilerast.ValuePattern, *compilerast.WildcardPattern:
		return nil, nil
	case *compilerast.CapturePattern:
		return []matchCapture{{name: pattern.Name, span: pattern.Span()}}, nil
	case *compilerast.AsPattern:
		var captures []matchCapture
		var err error
		if pattern.Pattern != nil {
			captures, err = compiler.basicPatternCaptures(pattern.Pattern)
			if err != nil {
				return nil, err
			}
		}
		if pattern.Name != "" {
			captures = append(captures, matchCapture{
				name: pattern.Name,
				span: pattern.Span(),
			})
		}
		return captures, nil
	case *compilerast.OrPattern:
		if len(pattern.Patterns) == 0 {
			return nil, compiler.error(pattern.Span(), "OR pattern has no alternatives")
		}
		for _, alternative := range pattern.Patterns {
			if _, err := compiler.basicPatternCaptures(alternative); err != nil {
				return nil, err
			}
		}
		return compiler.basicPatternCaptures(pattern.Patterns[0])
	default:
		return nil, compiler.unsupported(pattern)
	}
}
