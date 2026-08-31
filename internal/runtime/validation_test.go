package runtime_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler/bytecode"
	"github.com/spachava753/bullsnake/internal/compiler/lexer"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

func TestBytecodeValidation(t *testing.T) {
	aliasValueCode := testCode(
		1,
		[]bytecode.Instruction{
			{Opcode: bytecode.LoadConst},
			{Opcode: bytecode.ReturnValue},
		},
		[]bytecode.Constant{bytecode.None()},
		nil,
	)
	tests := []struct {
		name         string
		code         *bytecode.Code
		wantFragment string
	}{
		{
			name: "empty exception range",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				ExceptionHandlers: []bytecode.ExceptionHandler{
					{Start: 1, End: 1, Target: 1},
				},
			}),
			wantFragment: "exception handler 0 has empty or reversed range",
		},
		{
			name: "exception range end",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				ExceptionHandlers: []bytecode.ExceptionHandler{
					{Start: 0, End: 3, Target: 1},
				},
			}),
			wantFragment: "exception handler 0 range end 3 out of range",
		},
		{
			name: "exception handler target",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				ExceptionHandlers: []bytecode.ExceptionHandler{
					{Start: 0, End: 1, Target: 2},
				},
			}),
			wantFragment: "exception handler 0 target 2 out of range",
		},
		{
			name: "exception handler stack capacity",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				ExceptionHandlers: []bytecode.ExceptionHandler{
					{Start: 0, End: 1, Target: 1, StackDepth: 1},
				},
			}),
			wantFragment: "stack depth 1 cannot receive an exception with stack size 1",
		},
		{
			name: "overlapping exception ranges",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				ExceptionHandlers: []bytecode.ExceptionHandler{
					{Start: 0, End: 1, Target: 1},
					{Start: 0, End: 1, Target: 1},
				},
			}),
			wantFragment: "exception handler 1 overlaps or is out of order",
		},
		{
			name: "exception restore depth",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
					{Opcode: bytecode.PopTop},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				ExceptionHandlers: []bytecode.ExceptionHandler{
					{Start: 0, End: 1, Target: 2, StackDepth: 1},
				},
			}),
			wantFragment: "exception handler stack depth 1 exceeds instruction depth 0",
		},
		{
			name: "exception match underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CheckExceptionMatch},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "exception match left operand",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CheckExceptionMatch},
					{Opcode: bytecode.PopTop},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "CHECK_EXC_MATCH left operand is not an exception",
		},
		{
			name: "exception group match underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CheckExceptionGroupMatch},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "exception group match left operand",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadGlobal},
					{Opcode: bytecode.CheckExceptionGroupMatch},
					{Opcode: bytecode.PopTop},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				[]string{"Exception"},
			),
			wantFragment: "CHECK_EG_MATCH left operand is not an exception or None",
		},
		{
			name: "exception group merge underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.PrepareReraiseStar},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "exception group merge original value",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildList},
					{Opcode: bytecode.PrepareReraiseStar},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "PREP_RERAISE_STAR original value is not an exception",
		},
		{
			name: "exception group merge result value",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadGlobal},
					{Opcode: bytecode.Call},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.PrepareReraiseStar},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				[]string{"Exception"},
			),
			wantFragment: "PREP_RERAISE_STAR result value is not a list",
		},
		{
			name: "exception group merge result item",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadGlobal},
					{Opcode: bytecode.Call},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildList, Operand: 1},
					{Opcode: bytecode.PrepareReraiseStar},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				[]string{"Exception"},
			),
			wantFragment: "PREP_RERAISE_STAR result item 0 is not an exception or None",
		},
		{
			name: "reraise value",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Reraise},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "RERAISE value is not an exception",
		},
		{
			name: "exception scope target",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.EnterExcept},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "exception scope end 0 must follow its entry and stay within code",
		},
		{
			name: "exception scope underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.EnterExcept, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "exception scope value",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.EnterExcept, Operand: 3},
					{Opcode: bytecode.LeaveExcept},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "ENTER_EXCEPT value is not an exception",
		},
		{
			name: "exception scope leave",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LeaveExcept},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "LEAVE_EXCEPT has no active handler",
		},
		{
			name: "unsupported raise operand",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.RaiseVarargs, Operand: 3},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "unsupported RAISE_VARARGS operand 3",
		},
		{
			name: "exception scope merge",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.PopJumpIfFalse, Operand: 4},
					{Opcode: bytecode.LoadNotImplementedError},
					{Opcode: bytecode.EnterExcept, Operand: 6},
					{Opcode: bytecode.Nop},
					{Opcode: bytecode.LeaveExcept},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Bool(false), bytecode.None()},
				nil,
			),
			wantFragment: "exception scope mismatch at instruction 4",
		},
		{
			name: "function child index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "child code index 0 out of range",
		},
		{
			name: "fast local index",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadFast, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				Locals: []string{"value"},
			}),
			wantFragment: "local index 1 out of range",
		},
		{
			name: "global name index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadGlobal},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "name index 0 out of range",
		},
		{
			name: "specified format underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.FormatWithSpec},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("value")},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "import name index",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.ImportName},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("0"), bytecode.None()},
				nil,
			),
			wantFragment: "name index 0 out of range",
		},
		{
			name: "invalid formatted conversion",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ConvertValue, Operand: 99},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "unsupported CONVERT_VALUE operand 99",
		},
		{
			name: "formatted join underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildString, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("part")},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "call underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Call, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "invalid nested child",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.StoreName},
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.Integer("1")},
				Names:     []string{"visible"},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadLocals},
							{Opcode: bytecode.LoadFromDictOrGlobals, Operand: 1},
							{Opcode: bytecode.ReturnValue},
						},
						nil,
						[]string{"attribute"},
					),
				},
			}),
			wantFragment: "name index 1 out of range",
		},
		{
			name: "generator metadata",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Flags:     bytecode.Generator,
			}),
			wantFragment: "generator code requires optimized new locals",
		},
		{
			name: "yield outside generator code",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.YieldValue},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "YIELD_VALUE requires generator code",
		},
		{
			name: "send outside generator code",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Send, Operand: 3},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "SEND requires generator code",
		},
		{
			name: "send target",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Send, Operand: 99},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Flags:     bytecode.Optimized | bytecode.NewLocals | bytecode.Generator,
			}),
			wantFragment: "jump target 99 out of range",
		},
		{
			name: "send stack underflow",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Send, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Flags:     bytecode.Optimized | bytecode.NewLocals | bytecode.Generator,
			}),
			wantFragment: "operand stack underflow",
		},
		{
			name: "generator module code",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.YieldValue},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Flags:     bytecode.Optimized | bytecode.NewLocals | bytecode.Generator,
			}),
			wantFragment: "module code cannot be a generator",
		},
		{
			name: "yield stack underflow",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.YieldValue},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Flags:     bytecode.Optimized | bytecode.NewLocals | bytecode.Generator,
			}),
			wantFragment: "operand stack underflow",
		},
		{
			name: "function parameters exceed locals",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants:       []bytecode.Constant{bytecode.None()},
				Flags:           bytecode.Optimized | bytecode.NewLocals,
				PositionalCount: 1,
			}),
			wantFragment: "positional parameter count 1 exceeds local table length 0",
		},
		{
			name: "variadic positional local index",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants:       []bytecode.Constant{bytecode.None()},
				Flags:           bytecode.Optimized | bytecode.NewLocals | bytecode.VarArgs,
				PositionalCount: 1,
				Locals:          []string{"first"},
			}),
			wantFragment: "variadic positional parameter index 1 out of range",
		},
		{
			name: "unsupported unpacked call operand",
			code: testCode(
				0,
				[]bytecode.Instruction{
					{Opcode: bytecode.CallEx, Operand: 2},
				},
				nil,
				nil,
			),
			wantFragment: "unsupported CALL_EX operand 2",
		},
		{
			name: "invalid unpacked call payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CallEx, Operand: bytecode.CallExNoKeywords},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "CALL_EX positional arguments are not a tuple",
		},
		{
			name: "invalid keyword call payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 3,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.BuildTuple},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CallEx, Operand: bytecode.CallExWithKeywords},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "CALL_EX keyword arguments are not a dictionary",
		},
		{
			name: "map merge underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap},
					{Opcode: bytecode.MapMerge},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "variadic keyword local range",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Flags:     bytecode.Optimized | bytecode.NewLocals | bytecode.VarKeywords,
			}),
			wantFragment: "variadic keyword parameter index 0 out of range",
		},
		{
			name: "keyword-only local range",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				Constants:        []bytecode.Constant{bytecode.None()},
				Flags:            bytecode.Optimized | bytecode.NewLocals,
				KeywordOnlyCount: 1,
			}),
			wantFragment: "keyword-only parameter range exceeds local table length",
		},
		{
			name: "invalid keyword defaults payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionKeywordDefaults),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "function keyword defaults payload is not a dictionary",
		},
		{
			name: "deref index",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadDeref, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				Flags:  bytecode.Optimized | bytecode.NewLocals,
				Locals: []string{"value"},
				Cells:  []string{"value"},
			}),
			wantFragment: "deref index 1 out of range",
		},
		{
			name: "invalid closure payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionClosure),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCodeSpec(bytecode.CodeSpec{
						StackSize: 1,
						Instructions: []bytecode.Instruction{
							{Opcode: bytecode.LoadDeref},
							{Opcode: bytecode.ReturnValue},
						},
						Flags:    bytecode.Optimized | bytecode.NewLocals | bytecode.Nested,
						FreeVars: []string{"captured"},
					}),
				},
			}),
			wantFragment: "function closure payload is not a tuple",
		},
		{
			name: "closure cell count",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.BuildTuple},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionClosure),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Children: []*bytecode.Code{
					testCodeSpec(bytecode.CodeSpec{
						StackSize: 1,
						Instructions: []bytecode.Instruction{
							{Opcode: bytecode.LoadDeref},
							{Opcode: bytecode.ReturnValue},
						},
						Flags:    bytecode.Optimized | bytecode.NewLocals | bytecode.Nested,
						FreeVars: []string{"captured"},
					}),
				},
			}),
			wantFragment: "function closure has 0 cells for 1 free variables",
		},
		{
			name: "invalid annotate payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionAnnotate),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "function annotate payload is not a function",
		},
		{
			name: "unsupported function attribute",
			code: testCode(
				0,
				[]bytecode.Instruction{
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: 32,
					},
				},
				nil,
				nil,
			),
			wantFragment: "unsupported SET_FUNCTION_ATTRIBUTE operand 32",
		},
		{
			name: "function defaults underflow",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 1,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionDefaults),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "operand stack underflow",
		},
		{
			name: "invalid function defaults payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{
						Opcode:  bytecode.SetFunctionAttribute,
						Operand: uint32(bytecode.FunctionDefaults),
					},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children: []*bytecode.Code{
					testCode(
						1,
						[]bytecode.Instruction{
							{Opcode: bytecode.LoadConst},
							{Opcode: bytecode.ReturnValue},
						},
						[]bytecode.Constant{bytecode.None()},
						nil,
					),
				},
			}),
			wantFragment: "function defaults payload is not a tuple",
		},
		{
			name: "set build underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildSet, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "set element count",
			code: testCode(
				0,
				[]bytecode.Instruction{{Opcode: bytecode.BuildSet, Operand: 1}},
				nil,
				nil,
			),
			wantFragment: "set element count 1 exceeds stack size",
		},
		{
			name: "set add underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildSet},
					{Opcode: bytecode.SetAdd},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "set update underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildSet},
					{Opcode: bytecode.SetUpdate},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map set underflow",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MapSet},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map update underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap},
					{Opcode: bytecode.MapUpdate},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "store subscript underflow",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.StoreSubscript},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "delete subscript underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.DeleteSubscript},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map build underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildMap, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "map item count",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildMap, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "map item count 1 exceeds stack size",
		},
		{
			name: "mapping match underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.MatchMapping},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "mapping key match underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MatchMappingKey},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "mapping copy underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.CopyMapping},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "mapping duplicate check underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CheckMappingKey},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "type alias construction underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeTypeAlias},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("Alias")},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "type alias value payload",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.MakeTypeAlias},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("Alias"), bytecode.None()},
				nil,
			),
			wantFragment: "type alias value payload is not a function",
		},
		{
			name: "type variable construction underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.MakeTypeVar},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "type variable name payload",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeTypeVar},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "type variable name is not a string",
		},
		{
			name: "type alias parameter attachment underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.SetTypeAliasParameters},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "type alias parameter target",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildTuple},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.SetTypeAliasParameters},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "type alias parameter target is not a type alias",
		},
		{
			name: "type alias parameters payload",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 3,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.MakeTypeAlias},
					{Opcode: bytecode.SetTypeAliasParameters},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{
					bytecode.None(),
					bytecode.TextString("Alias"),
				},
				Children: []*bytecode.Code{aliasValueCode},
			}),
			wantFragment: "type alias parameters payload is not a tuple",
		},
		{
			name: "type alias parameter member",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 3,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildTuple, Operand: 1},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.MakeTypeAlias},
					{Opcode: bytecode.SetTypeAliasParameters},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{
					bytecode.None(),
					bytecode.TextString("Alias"),
				},
				Children: []*bytecode.Code{aliasValueCode},
			}),
			wantFragment: "type alias parameter payload contains a non-TypeVar value",
		},
		{
			name: "type variable bound attachment underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeTypeVar},
					{Opcode: bytecode.SetTypeVarBound},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("T")},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "type variable constraints attachment underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeTypeVar},
					{Opcode: bytecode.SetTypeVarConstraints},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("T")},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "type variable default attachment underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeTypeVar},
					{Opcode: bytecode.SetTypeVarDefault},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("T")},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "type variable evaluator payload",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeTypeVar},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.SetTypeVarBound},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("T"), bytecode.None()},
				nil,
			),
			wantFragment: "type variable evaluator payload is not a function",
		},
		{
			name: "type variable evaluator target",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 2,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.SetTypeVarBound},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children:  []*bytecode.Code{aliasValueCode},
			}),
			wantFragment: "type variable evaluator target is not a TypeVar",
		},
		{
			name: "class match underflow",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.MatchClass},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "class positional pattern count",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.MatchClass, Operand: 2},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "class positional pattern count 2 exceeds stack size",
		},
		{
			name: "sequence match underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.MatchSequence},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "sequence length underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.GetLen},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "starred unpack underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.UnpackEx},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "starred unpack count",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.UnpackEx, Operand: 1 | 1<<8},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "unpack count 3 exceeds stack size",
		},
		{
			name: "list append underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildList},
					{Opcode: bytecode.ListAppend},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "list extend underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.BuildList},
					{Opcode: bytecode.ListExtend},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "list to tuple underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.ListToTuple},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "invalid build slice operand",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildSlice, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "unsupported BUILD_SLICE operand 1",
		},
		{
			name: "build slice underflow",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildSlice, Operand: 3},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "binary subscript underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BinarySubscript},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "get iterator underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.GetIter},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "for iterator underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.ForIter, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "for iterator jump target",
			code: testCode(
				1,
				[]bytecode.Instruction{{Opcode: bytecode.ForIter, Operand: 1}},
				nil,
				nil,
			),
			wantFragment: "jump target 1 out of range",
		},
		{
			name: "sequence build underflow",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BuildTuple, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "copy depth",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Copy},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "COPY depth must be at least 1",
		},
		{
			name: "swap depth",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Swap, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "SWAP depth must be at least 2",
		},
		{
			name: "unsupported comparison",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.CompareOp, Operand: 99},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				nil,
			),
			wantFragment: "unsupported COMPARE_OP operand 99",
		},
		{
			name: "jump target",
			code: testCode(
				0,
				[]bytecode.Instruction{{Opcode: bytecode.Jump, Operand: 1}},
				nil,
				nil,
			),
			wantFragment: "jump target 1 out of range",
		},
		{
			name: "conditional stack underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.PopJumpIfFalse, Operand: 1},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "stack depth merge",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.PopJumpIfFalse, Operand: 4},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Jump, Operand: 5},
					{Opcode: bytecode.Nop},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "stack depth mismatch at instruction 5",
		},
		{
			name: "no reachable return",
			code: testCode(
				0,
				[]bytecode.Instruction{{Opcode: bytecode.Jump}},
				nil,
				nil,
			),
			wantFragment: "code has no reachable RETURN_VALUE",
		},
		{
			name: "unknown opcode",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.Opcode(255)},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				nil,
			),
			wantFragment: "unsupported opcode Opcode(255)",
		},
		{
			name: "class annotation lookup underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadFromDictOrGlobals},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				[]string{"value"},
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "class annotation namespace type",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadFromDictOrGlobals},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				[]string{"value"},
			),
			wantFragment: "class annotation namespace is not a namespace",
		},
		{
			name: "class annotation dereference out of range",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadLocals},
					{Opcode: bytecode.LoadFromDictOrDeref},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "deref index 0 out of range",
		},
		{
			name: "special method name out of range",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadSpecial},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "name index 0 out of range",
		},
		{
			name: "handled exception type outside handler",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadHandledExceptionType},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantFragment: "LOAD_HANDLED_EXCEPTION_TYPE has no active exception",
		},
		{
			name: "unsupported matrix binary operation",
			code: testCode(
				2,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.BinaryOp, Operand: bytecode.BinaryMatrixMultiply},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				nil,
			),
			wantFragment: "unsupported BINARY_OP operand 3",
		},
		{
			name: "unsupported unary operation",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.UnaryOp, Operand: 99},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("1")},
				nil,
			),
			wantFragment: "unsupported UNARY_OP operand 99",
		},
		{
			name: "unsupported constant",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{{Kind: bytecode.ConstantKind(255)}},
				nil,
			),
			wantFragment: "unsupported constant Constant(kind=255)",
		},
		{
			name: "invalid integer constant",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.Integer("not-an-integer")},
				nil,
			),
			wantFragment: "invalid integer",
		},
		{
			name: "invalid string encoding",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.TextString("\xff")},
				nil,
			),
			wantFragment: "invalid string constant encoding",
		},
		{
			name: "constant index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "constant index 1 out of range",
		},
		{
			name: "name index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadName, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				[]string{"present"},
			),
			wantFragment: "name index 1 out of range",
		},
		{
			name: "attribute name index",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadAttr, Operand: 1},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				[]string{"present"},
			),
			wantFragment: "name index 1 out of range",
		},
		{
			name: "attribute stack underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadAttr},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				[]string{"attribute"},
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "stack underflow",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.PopTop},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack underflow",
		},
		{
			name: "declared stack too small",
			code: testCode(
				0,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None()},
				nil,
			),
			wantFragment: "operand stack exceeds declared size 0",
		},
		{
			name:         "fallthrough",
			code:         testCode(0, []bytecode.Instruction{{Opcode: bytecode.Nop}}, nil, nil),
			wantFragment: "code falls through without RETURN_VALUE",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := bullruntime.New()
			module, err := runtime.ExecuteModule("broken", test.code)
			if module != nil {
				t.Fatalf("module = %#v, want nil", module)
			}
			var invalid *bullruntime.BytecodeError
			if !errors.As(err, &invalid) {
				t.Fatalf("error = %T %v, want *runtime.BytecodeError", err, err)
			}
			if !strings.Contains(invalid.Error(), test.wantFragment) {
				t.Fatalf("error = %q, want fragment %q", invalid, test.wantFragment)
			}
			if _, ok := runtime.Module("broken"); ok {
				t.Fatal("invalid module entered the runtime cache")
			}
		})
	}
}

func TestAnnotationFormatRejection(t *testing.T) {
	code := testCode(
		1,
		[]bytecode.Instruction{
			{Opcode: bytecode.LoadNotImplementedError},
			{Opcode: bytecode.RaiseVarargs, Operand: 1},
		},
		nil,
		nil,
	)
	_, err := bullruntime.New().ExecuteModule("annotation format", code)
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != "NotImplementedError" {
		t.Errorf("exception type = %q, want NotImplementedError", got)
	}
	if got := raised.Exception().Message(); got != "" {
		t.Errorf("exception message = %q, want empty", got)
	}
}

func TestClassBuilderArguments(t *testing.T) {
	body := testCode(
		1,
		[]bytecode.Instruction{
			{Opcode: bytecode.LoadConst},
			{Opcode: bytecode.ReturnValue},
		},
		[]bytecode.Constant{bytecode.None()},
		nil,
	)
	tests := []struct {
		name        string
		code        *bytecode.Code
		wantMessage string
	}{
		{
			name: "too few",
			code: testCode(
				1,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadBuildClass},
					{Opcode: bytecode.Call},
					{Opcode: bytecode.ReturnValue},
				},
				nil,
				nil,
			),
			wantMessage: "__build_class__: not enough arguments",
		},
		{
			name: "body is not function",
			code: testCode(
				3,
				[]bytecode.Instruction{
					{Opcode: bytecode.LoadBuildClass},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.LoadConst, Operand: 1},
					{Opcode: bytecode.Call, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				[]bytecode.Constant{bytecode.None(), bytecode.TextString("Broken")},
				nil,
			),
			wantMessage: "__build_class__: func must be a function",
		},
		{
			name: "name is not string",
			code: testCodeSpec(bytecode.CodeSpec{
				StackSize: 3,
				Instructions: []bytecode.Instruction{
					{Opcode: bytecode.LoadBuildClass},
					{Opcode: bytecode.MakeFunction},
					{Opcode: bytecode.LoadConst},
					{Opcode: bytecode.Call, Operand: 2},
					{Opcode: bytecode.ReturnValue},
				},
				Constants: []bytecode.Constant{bytecode.None()},
				Children:  []*bytecode.Code{body},
			}),
			wantMessage: "__build_class__: name is not a string",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := bullruntime.New().ExecuteModule("class error", test.code)
			var raised *bullruntime.UncaughtException
			if !errors.As(err, &raised) {
				t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
			}
			if got := raised.Exception().TypeName(); got != "TypeError" {
				t.Errorf("exception type = %q, want TypeError", got)
			}
			if got := raised.Exception().Message(); got != test.wantMessage {
				t.Errorf("exception message = %q, want %q", got, test.wantMessage)
			}
		})
	}
}

func testCode(
	stackSize int,
	instructions []bytecode.Instruction,
	constants []bytecode.Constant,
	names []string,
) *bytecode.Code {
	return testCodeSpec(bytecode.CodeSpec{
		StackSize:    stackSize,
		Instructions: instructions,
		Constants:    constants,
		Names:        names,
	})
}

func testCodeSpec(spec bytecode.CodeSpec) *bytecode.Code {
	positions := make([]lexer.Span, len(spec.Instructions))
	for index := range positions {
		positions[index] = lexer.Span{
			Start: lexer.Position{Line: 1, Column: index},
			End:   lexer.Position{Line: 1, Column: index + 1},
		}
	}
	if spec.Filename == "" {
		spec.Filename = "<broken>"
	}
	if spec.Name == "" {
		spec.Name = "<module>"
	}
	if spec.QualifiedName == "" {
		spec.QualifiedName = spec.Name
	}
	if spec.FirstLine == 0 {
		spec.FirstLine = 1
	}
	spec.Positions = positions
	return bytecode.NewCode(spec)
}
