# Bytecode package notes

Before changing bytecode, read the `Compiler and bytecode`, `Virtual machine and
frames`, and `Testing` sections in `docs/impl.md`.

## Package contract

This package defines Bullsnake's private instructions, constants, and immutable
code objects. It is the data passed from the compiler to the runtime. It is not a
Python API, a saved file format, or a compatibility promise between releases.

Code objects copy mutable input and return copies of their tables. Keep them
safe to cache and share inside one process. Do not add a bytecode version until
Bullsnake has a real saved or external format that can cross builds.

The compiler may define and emit an instruction before the runtime supports it.
That is expected while the interpreter is being built. Runtime preparation must
reject every unsupported instruction in the complete code tree before execution
starts, including unreachable instructions and child code.

Never remove or relax a runtime guard merely because the parser accepts a form,
the compiler emits an instruction, or the instruction is unlikely to run. Add
runtime acceptance only as part of the chosen runtime feature, with its behavior
and failure tests.

An instruction's compiler stack effect, printed form, operand meaning, and source
position belong to the bytecode contract. Runtime validation may need additional
stack rules for jumps and other instructions with more than one path.

This package must not import the runtime or store runtime values.

## Testing

Use focused tests for code-object copying, opcode names, operands, packed values,
and compiler stack effects. Compiler cases prove emitted instruction sequences.
Runtime validation tests prove which instructions and operands the VM currently
accepts.

Defining an instruction and executing it may land in different commits. Keep the
runtime rejection test until the execution commit is complete.
