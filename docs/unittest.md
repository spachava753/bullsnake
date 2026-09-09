# First major milestone: run Python's unittest

Status: language groundwork is in place; importing and running `unittest` is
still blocked.

The goal is to run CPython's unchanged synchronous `unittest` package, then use
it to run unchanged standard-library tests. Much of the Python language support
needed for this work already exists. The next phase needs Go-backed system
modules and more runtime behavior, especially class creation, weak references,
and tracebacks that Python code can inspect.

This document tracks that milestone. The [architecture](architecture.md)
explains the interpreter's design, and the [implementation guide](impl.md)
describes its full supported language subset.

## What we want to run

The reference is CPython 3.14.7 at commit
`823f0323ee6ec1402088b73bce1a38473cac36dc`. Keep its Python source unchanged so
passing tests demonstrate compatibility with the real framework.

The synchronous milestone covers:

- `TestCase` and `TestResult`: run test methods and record passes, failures,
  errors, skips, and expected failures, including setup, teardown, cleanups,
  and subtests.
- `TestSuite` and `TestLoader`: group tests and select them by name or from
  modules that are already imported.
- `TextTestRunner`: write the test report to `io.StringIO`, a text stream held
  in memory.
- CPython's unchanged `test_colorsys.py`: replace our adapted assertions with
  the original tests running under `unittest`.
- `unittest.main()`: use arguments and output streams supplied to the runtime,
  without taking over the embedding Go program's process.

Finding tests by scanning directories and handling Ctrl-C during a test run come
later. `unittest.mock` needs more support for inspecting and changing objects.
`IsolatedAsyncioTestCase` needs an event loop, task context, and cancellation.
Neither is part of this milestone.

## Where we are now

### Implemented and tested in this repository

| Area | Progress relevant to unittest |
| --- | --- |
| Python execution | Functions, all parameter kinds, decorators, closures, classes, inheritance, loops, comprehensions, generators, exceptions, and context managers have execution tests. |
| Objects and builtins | Method binding, properties, `super`, attribute helpers, type checks, user-defined iteration and comparisons, sorting, and many string and collection methods work within the documented subset. |
| Imports | Source modules, regular packages, relative imports, repeated and circular imports, and cleanup after a failed import are tested. |
| Go-backed modules | Each runtime starts with `builtins`, `__future__`, `_functools.cmp_to_key`, and `string.templatelib`, plus its `string` parent package. General Go module registration and system modules are still missing. |
| Standard-library tests | Unchanged `colorsys.py` runs with adapted versions of all eight upstream public test methods. These use plain assertions, not `TestCase` objects. |

The [language tests](../internal/runtime/testdata/execution/) run Python source
through parsing, name resolution, compilation, bytecode validation, and
execution. The [standard-library runner](../stdlib/stdlib_test.go) uses the
filesystem loader and creates a fresh runtime for each test module.

The checked-in standard-library tree contains only `colorsys.py` and its adapted
test. It does not yet contain `unittest` or its dependencies.

### Results from the earlier compatibility probes

The last recorded sweep compiled all 61 Python source modules observed while a
pinned CPython process imported synchronous `unittest`. The parser, name
resolver, and compiler handled those files. This does not show that every
function in them can execute.

The last recorded attempt to execute the unchanged package stopped at:

```text
Lib/io.py:53:8: ModuleNotFoundError: No module named '_io'
```

Separate import probes at the same revision reported:

| Module | Result |
| --- | --- |
| `operator` | Imported using its unchanged Python fallback. |
| `keyword` | Imported unchanged. Its lookup helpers use frozen-set containment methods. |
| `heapq` | Imported using its unchanged Python fallback. |
| `abc` | Reached `Lib/_weakrefset.py:5:1`, then failed because `_weakref` was missing. |

These are recorded probe results, not checks in the current Go test suite.
Repeat them against the pinned source when implementation resumes, and turn
working imports into checked-in regression tests. An import success alone does
not establish that a module's public functions work.

The evidence supports moving on to system-module work. It does not establish
that all the language features needed by `unittest` are finished.

## What to do next

### 1. Decide how Go-backed modules get their state and permissions

This is the current pause point. Before exposing system modules, decide how the
embedding Go program supplies or denies access to host resources. A public Go
API is not required yet, but the internal design must leave these choices with
the host.

Add an internal way to create Go-backed modules for each runtime. Use the same
module cache and attribute behavior as Python modules, and keep mutable state
isolated between runtimes.

| Module or service | Decision needed |
| --- | --- |
| `sys` | How to expose the live module cache, arguments, replaceable streams, active exceptions, exit requests, and selected platform information. |
| `_io` | Start with in-memory streams. Decide separately how `open`, `FileIO`, descriptors, buffering, and text encoding will work. |
| `time` | How the host supplies or permits the clock used by `perf_counter`. |
| `signal` | What can be imported without installing handlers, and how a host opts into process signal access later. |
| `os`, `posix`, `stat`, `errno` | What metadata and path behavior imports need, and how filesystem and process access will be configured before discovery is enabled. |

The existing Go filesystem loader can read Python source files. It does not give
Python code an `open` builtin or an `os` module.

### 2. Make the unchanged package import

Start with `abc` and the class behavior it needs, then in-memory `_io` and
unchanged `io.py`. Continue through the other dependencies as each import
reveals the next missing operation. Adding `_io` alone will not make
`import unittest` succeed.

For `StringIO`, cover `write`, `writelines`, `flush`, `seek`, `tell`, `truncate`,
`getvalue`, `close`, `closed`, and use in a `with` statement. Provide the stream
information used by `_colorize`, including a defined `isatty` result. Test that
Python code can replace `sys.stdout` and `sys.stderr`.

The checkpoint is an exact `import unittest` through the normal loader, with
its dependencies present and no edits that bypass import-time work.

### 3. Run tests and check their results

Run one passing test through `TestCase` and `TestResult`, then one assertion
failure and one unexpected exception. Add separate tests for setup and teardown,
cleanups, skips, expected failures, subtests, and traceback formatting.

This will expose operations that successful imports never exercised. Implement
each missing behavior with a focused test rather than trying to finish every
dependency module in advance.

### 4. Load suites, produce reports, and replace the adapted test

Run `TestSuite` and name-based `TestLoader` cases without directory discovery.
Then run `TextTestRunner` against `StringIO` and compare the complete report.
Replace the adapted colorsys test with unchanged CPython `test_colorsys.py` once
these paths work. Finally, exercise `unittest.main()` with runtime-owned
arguments and streams, without terminating the Go test process.

Go's `testing` package remains the outer runner. Python's `unittest` should own
test selection, assertions, results, and report formatting. Keep each finished
change small, add its behavior tests, update the implementation notes, and run
the repository checks before committing it.

## Blockers and lessons so far

### The package imports more than a small test uses

`unittest.result` imports `io`, `sys`, and `traceback` before defining
`TestResult`. Unchanged `io.py` imports constants, exceptions, functions, stream
types, and base classes from `_io`. Adding only a `StringIO` export cannot make
that source work.

Other imports bring in further dependencies:

| Import path | Dependencies that matter |
| --- | --- |
| `unittest.result` | Streams, active exception state, and traceback formatting. |
| `unittest.case` | `functools`, `difflib`, `pprint`, `re`, `warnings`, `collections`, `contextlib`, `traceback`, `time`, and `types`. |
| `unittest.runner` | Text streams, `time.perf_counter`, warnings, and `_colorize`. |
| `unittest.signals` | `signal` and a `weakref.WeakKeyDictionary` created during import. |
| `unittest.loader` and `unittest.main` | `sys`, `os`, path helpers, argument parsing, and discovery code. |

Supporting these imports does not require enabling every filesystem or signal
operation immediately. It does require real behavior for anything executed
during import. Bullsnake also validates every compiled function before a
module runs, including unused functions. Unsupported bytecode cannot be hidden
there; missing attributes and call behavior may still fail only when executed.

### I/O also needs abstract classes and metaclasses

An abstract base class, or ABC, can require subclasses to implement methods and
can register other classes as accepted subclasses. A metaclass controls how a
class is created. Unchanged `io.py` uses both through `abc.ABCMeta` to define
`IOBase`, `RawIOBase`, `BufferedIOBase`, and `TextIOBase`.

Bullsnake can now define classes that inherit `classmethod`, `staticmethod`, or
`property`. This lets `abc.py` get past its older descriptor helper definitions.
Constructing instances of those subclasses still fails. Subclassing `type`,
selecting `metaclass=`, and calling metaclass `__new__` are also unsupported.

Choose between implementing the `_abc` helper used by `abc.py` and supporting
its `_py_abc` fallback through `_weakref`, `weakref`, and `_weakrefset`. Either
route still needs class creation, abstract-method checks, subclass registration,
and the matching behavior in `isinstance` and `issubclass`.

### Weak references need a memory and callback design

A weak reference lets code refer to an object without keeping it alive.
`unittest.signals` creates a `WeakKeyDictionary` even when Ctrl-C handling is
unused, so choosing `_abc` does not remove the package's weak-reference need.

A dictionary that holds strong references would keep test results alive and
would not reproduce weak-reference callbacks. Bullsnake uses Go's garbage
collector. Any Python callback triggered by collection must wait until the
interpreter can safely run it, rather than running inside a Go cleanup callback.
Settle that design before exposing `_weakref`.

### Failure reports need Python-visible traceback objects

The runtime already retains traceback data for Go callers, handles exception
causes and context, and supports `exception.with_traceback(None)`. That method
clears retained traceback entries and returns the same exception. It does not
provide a traceback object to Python.

The remaining work includes `sys.exc_info()` and `sys.exception()`, exception
`args` and mutable `__traceback__` state, and Python objects for traceback links,
frames, code information, source positions, and frame clearing. Python and Go
must see consistent views of the same exception and frame data.

Use this support to run the needed `traceback.TracebackException`, `format_exc`,
and `clear_frames` behavior. Add the exception and warning classes required by
the executed dependencies.

### Objects and builtins still have gaps

Known gaps to check as execution advances are:

- Custom attribute lookup through `__getattribute__` and `__getattr__`, plus
  more readable and writable class and function metadata.
- Construction of native-type subclasses, including descriptors, metaclasses,
  and the container subclasses needed by code such as `namedtuple`.
- Dictionary and set keys whose hashing or equality calls Python methods.
  Direct `hash(obj)` supports a user method today, but container keys do not.
- List, set, and frozen-set equality involving user-defined element equality.
  These containers currently use fixed runtime comparisons.
- Additional collection, string, bytes, and bytearray methods, and builtin call
  forms such as `max(iterable)` and `min(iterable)`.

Two existing differences may also matter to tests: `list.sort` does not expose
an empty list to callbacks or detect mutation during sorting as CPython does,
and string casing uses Go's Unicode 17 tables rather than CPython 3.14's Unicode
16 baseline. Keep these differences visible when a test reaches them.

### Prefer Python fallbacks where they work

The `operator` and `heapq` probes show that their native accelerators are not
needed just to import them. `_functools.cmp_to_key` already exists as a small
Go-backed helper; other `_functools` exports remain absent so Python fallbacks
can run.

Add pinned source dependencies as tests reach them, including supporting modules
such as `fnmatch`, `inspect`, and `argparse`. Potential native helpers include
`_abc`, `_collections`, `_heapq`, `_operator`, `_sre`, `_warnings`, and selected
`itertools` operations. Implement a helper only when the Python fallback is
absent or cannot provide the required behavior. Every exposed operation needs
tests for its arguments, results, errors, and relevant object identity rules.

Regular expressions need a separate compatibility decision. Go's regexp package
cannot be assumed to reproduce Python patterns, flags, groups, match positions,
and substitutions.

## How we will know it is done

The synchronous in-memory milestone is complete when:

- The unchanged package and required dependencies import from the vendored
  CPython 3.14.7 source tree.
- Test methods, setup and teardown, cleanups, skips, expected failures,
  subtests, assertion failures, and unexpected exceptions produce the expected
  `TestResult` entries.
- Failure reports use Python-visible exception and traceback state.
- `TestSuite` and name-based `TestLoader` execution work.
- `TextTestRunner` writes the expected complete report to `StringIO`.
- Unchanged `test_colorsys.py` passes through the standard-library Go runner.
- `unittest.main()` uses the configured arguments and streams, and its exit
  request does not terminate the embedding Go process.
- All repository checks pass without network access or a Python executable.

After that, add permission-controlled filesystem discovery and signal handling.
Plan mock and async testing separately. The immediate next task remains the
runtime-owned module and host-access design.
