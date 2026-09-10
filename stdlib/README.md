# Vendored Python standard library

This directory contains selected Python standard-library modules and tests from
CPython 3.14.7, commit `823f0323ee6ec1402088b73bce1a38473cac36dc`.

Module files are copied unchanged. Test files contain only the upstream cases
Bullsnake intends to run. Each adapted test records its changes in the file.
The CPython license is in `LICENSES/CPython-3.14.txt`.

## Synchronous unittest checkpoint

The vendored `operator.py`, `keyword.py`, and `heapq.py` fallbacks now run
project-owned smoke tests through the ordinary source loader. These exercise
operator calls, keyword classification, and heap construction/push/pop. They do
not claim full module conformance.

The unchanged synchronous `unittest` sources are retained as the next execution
target: `__init__`, `__main__`, `case`, `loader`, `main`, `result`, `runner`,
`signals`, `suite`, and `util`. `io.py`, `abc.py`, `_py_abc.py`, and
`_weakrefset.py` are the first dependency sources reached by import probes.
These files come from `Lib/` at the revision above and have no modifications.
The complete transitive dependency closure is not vendored yet. Mock and async
unittest source/support are outside this checkpoint.

The framework does not import yet. From the repository root, reproduce the
current blockers without Python or network access:

```sh
go run ./tools/importprobe stdlib/3.14 abc unittest
```

`abc` now imports through native `_abc` helpers backed by Go weak pointers.
Project-owned `tests/test_abc.py` exercises abstract construction, descriptors,
virtual registration, hooks, and checks against unchanged abc.py. It is not the
upstream test_abc suite. A separate test checks the full unchanged
`_dump_registry` report through print, using explicit and redirected Python
streams. `update_abstractmethods` still needs class dictionary access. `unittest` stops
at `io.py:56:1` because `_collections_abc` is absent. Native `_io` now has
StringIO and the initial I/O base classes; the rest of its surface is incomplete.
The adapted colorsys tests remain
in place until unchanged unittest execution can replace them.
