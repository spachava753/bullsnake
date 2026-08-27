package ast

import (
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/lexer"
)

func TestDump(t *testing.T) {
	nameSpan := testSpan(0, 1, 1, 0, 1, 1)
	leftSpan := testSpan(4, 5, 1, 4, 1, 5)
	rightSpan := testSpan(8, 9, 1, 8, 1, 9)
	valueSpan := testSpan(4, 9, 1, 4, 1, 9)
	statementSpan := testSpan(0, 9, 1, 0, 1, 9)
	module := &Module{
		Range: statementSpan,
		Body: []Stmt{
			&AssignStmt{
				Range:   statementSpan,
				Targets: []Expr{&Name{Range: nameSpan, ID: "x", Context: Store}},
				Value: &BinaryExpr{
					Range: valueSpan,
					Left:  &NumberLiteral{Range: leftSpan, Text: "1"},
					Op:    Add,
					Right: &NumberLiteral{Range: rightSpan, Text: "2"},
				},
			},
		},
	}

	want := `Module(body=[AssignStmt(targets=[Name(id="x", context=Store)], value=BinaryExpr(left=NumberLiteral(text="1"), op=Add, right=NumberLiteral(text="2")))])`
	if got := Dump(module, DumpOptions{}); got != want {
		t.Fatalf("Dump() =\n%s\nwant:\n%s", got, want)
	}

	wantSpans := `Module(body=[AssignStmt(targets=[Name(id="x", context=Store)@1:0-1:1], value=BinaryExpr(left=NumberLiteral(text="1")@1:4-1:5, op=Add, right=NumberLiteral(text="2")@1:8-1:9)@1:4-1:9)@1:0-1:9])@1:0-1:9`
	if got := Dump(module, DumpOptions{IncludeSpans: true}); got != wantSpans {
		t.Fatalf("Dump(include spans) =\n%s\nwant:\n%s", got, wantSpans)
	}
}

func TestEnumStringsRemainDefinedOutsideKnownRange(t *testing.T) {
	if got := ExprContext(255).String(); got != "ExprContext(255)" {
		t.Fatalf("unknown expression context = %q", got)
	}
	if got := BinaryOperator(255).String(); got != "BinaryOperator(255)" {
		t.Fatalf("unknown binary operator = %q", got)
	}
	if got := ComparisonOperator(255).String(); got != "ComparisonOperator(255)" {
		t.Fatalf("unknown comparison operator = %q", got)
	}
}

func testSpan(startOffset, endOffset, startLine, startColumn, endLine, endColumn int) lexer.Span {
	return lexer.Span{
		Start: lexer.Position{Offset: startOffset, Line: startLine, Column: startColumn},
		End:   lexer.Position{Offset: endOffset, Line: endLine, Column: endColumn},
	}
}
