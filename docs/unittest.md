# Running CPython's `unittest`

Status: planned compatibility milestone

This document records what remains before Bullsnake can run the unchanged
synchronous `unittest` package from CPython 3.14.7. It complements the
[architecture](architecture.md), which explains long-term design choices, and
the [implementation guide](impl.md), which describes code that exists now.

## Target

The first target is the synchronous package imported by `unittest/__init__.py`:

- import the unchanged package and its eager dependencies
- run `TestCase`, `TestResult`, `TestSuite`, and `TestLoader`
- run `TextTestRunner` against an in-memory text stream
- execute `unittest.main()` with runtime-owned arguments and streams
- replace Bullsnake's adapted colorsys assertions with CPython's unchanged
  `test_colorsys.py`

`unittest.mock` and `IsolatedAsyncioTestCase` are later milestones. Mock needs a
larger reflection and attribute model. The async class needs an event loop,
context state, and cancellation behavior. Filesystem discovery and catch-break
signal handling also come after the in-memory synchronous runner.

## Reference and current result

The reference is CPython 3.14.7 at commit
`823f0323ee6ec1402088b73bce1a38473cac36dc`.

An exact frontend sweep compiles all 61 Python source modules observed while a
pinned CPython process imports the synchronous package. This covers the syntax,
name resolution, and bytecode construction needed by the current import graph.
Dormant functions may still expose runtime gaps when tests begin to call them.

Executing the unchanged package through Bullsnake's filesystem loader currently
stops here:

```text
Lib/io.py:53:8: ModuleNotFoundError: No module named '_io'
```

Focused probes at the same pinned revision currently produce:

```text
operator imported
keyword imported
heapq imported
abc -> Lib/_weakrefset.py:5:1: ModuleNotFoundError: No module named '_weakref'
```

The `operator`, `keyword`, and `heapq` results matter because they use their
unchanged pure-Python fallbacks. Bullsnake does not need native accelerator
modules merely to make those imports succeed.

The checked-in standard-library runner still uses adapted plain assertions for
`colorsys`. It does not yet execute Python `TestCase` objects.

## Why `_io` is only the first blocker

`unittest.result` imports `io`, `sys`, and `traceback` before defining
`TestResult`. The public `io` module immediately imports `_io`, then defines
Python ABC wrappers around native I/O base classes. Supplying only a `StringIO`
name would move the failure without supporting the module.

Other eager package paths add these requirements:

| Path | Required behavior |
| --- | --- |
| `unittest.result` | mutable `sys.stdout` and `sys.stderr`, `io.StringIO`, `sys.exc_info`, traceback formatting |
| `unittest.case` | `functools`, `difflib`, `pprint`, `re`, `warnings`, `collections`, `contextlib`, `traceback`, `time`, and `types` |
| `unittest.runner` | text streams, `time.perf_counter`, warnings, and `_colorize` |
| `unittest.signals` | `signal` and `weakref.WeakKeyDictionary` |
| `unittest.loader` and `unittest.main` | `sys`, `os`, path operations, argument parsing, and filesystem discovery |

The package imports all of these modules even when a small test does not call
every API they contain. Bullsnake validates complete code trees before a module
body runs, so unsupported code cannot be hidden in an unused function.

## Remaining runtime foundations

### Runtime-owned modules and host access

Bullsnake currently creates a few fixed modules directly inside `Runtime`. The
standard library needs a deliberate internal module-factory mechanism for
runtime-owned modules. The mechanism must use the ordinary module cache and
attribute rules, and it must keep mutable state on one `Runtime`.

The first host-facing modules need explicit policies:

- `sys` needs a live module cache, arguments, streams, exception state, exit
  behavior, and selected platform metadata.
- `_io` needs in-memory stream types for the first runner milestone. File-backed
  `open`, `FileIO`, descriptors, buffering, and encoding policy are required
  later for discovery.
- `time` needs a clock policy before exposing `perf_counter`.
- `signal` needs an opt-in host policy before reading or changing process signal
  handlers.
- `os`, `posix`, `stat`, and `errno` need a filesystem and process capability
  policy before discovery can run.

These APIs must not read or mutate ambient process state accidentally. A host
must be able to configure or deny them when Bullsnake gains a public runtime
constructor.

### I/O and stream objects

The unchanged `io.py` imports native constants, exceptions, functions, concrete
streams, and native base classes from `_io`. It then creates `IOBase`,
`RawIOBase`, `BufferedIOBase`, and `TextIOBase` with `abc.ABCMeta`.

The first in-memory runner needs:

- `StringIO` with `write`, `writelines`, `flush`, `seek`, `tell`, `truncate`,
  `getvalue`, `close`, `closed`, and context-manager behavior
- stream metadata used by `_colorize`, including the chosen `isatty` behavior
- mutable `sys.stdout` and `sys.stderr` references
- enough I/O ABC behavior for unchanged `io.py` to define and register classes

Filesystem streams can follow as a separate capability-backed milestone.

### ABCs, metaclasses, and weak references

Bullsnake now lets a user class inherit `classmethod`, `staticmethod`, or
`property`, which allows unchanged `abc.py` to define its deprecated descriptor
helpers. The next fallback import reaches `_weakrefset` and then `_weakref`.

There are two implementation paths, and both need real behavior:

- implement the `_abc` operations used by `abc.py`
- implement `_weakref`, `weakref`, and `_weakrefset` so `_py_abc` can run

Either path still needs user subclasses of `type`, class `metaclass=` selection,
metaclass `__new__` calls, abstract-method collection, virtual subclass
registration, and matching through `isinstance` and `issubclass`.

`unittest.signals` also constructs `weakref.WeakKeyDictionary` during import.
A strong-reference substitute would keep test results alive and would give the
wrong callback behavior. Weak callbacks must run at a safe VM point rather than
inside a Go cleanup callback.

### Python-visible exceptions, frames, and tracebacks

Bullsnake retains enough frame information for a Go host traceback. `unittest`
needs Python objects and mutable exception state:

- `sys.exc_info()` and `sys.exception()` tied to the active handled exception
- exception `args`, `__traceback__`, cause, context, and `with_traceback`
- traceback links, frame links, code metadata, source positions, and frame
  clearing
- `traceback.TracebackException`, `format_exc`, and `clear_frames`
- the exception and warning classes referenced by the dependency graph

The existing host traceback should remain one view of the same frame data, not
an independent record with different ordering.

### Object and container behavior

The frontend can compile the dependency graph, and the runtime already covers
many operations it uses. Execution tests will still need to drive the remaining
object behavior in small slices. Known areas include:

- custom `__getattribute__`, fallback `__getattr__`, and broader mutable class
  and function metadata
- construction of supported native-type subclasses, including descriptor and
  metaclass instances
- built-in container subclasses and the behavior needed by `namedtuple`
- dictionary and set hashing or equality that calls user methods
- remaining list, dictionary, set, string, bytes, and bytearray methods reached
  by executed code
- remaining builtin call forms such as iterable `max` and `min`, plus builtins
  selected by new execution probes

Placeholders are not progress. Each name needs argument binding, errors,
identity rules, and behavioral tests before it enters the builtin namespace.

### Pure-Python and helper modules

After the core runtime-owned modules exist, add pinned source modules in the
order exposed by import and execution tests. The synchronous package directly
uses behavior from:

```text
collections contextlib difflib fnmatch functools inspect io pprint
re time traceback types warnings weakref signal argparse _colorize
```

Some source modules need side-effect-free native helpers such as `_abc`,
`_collections`, `_functools`, `_heapq`, `_operator`, `_sre`, `_warnings`, and
selected `itertools` operations. Prefer an unchanged pure-Python fallback when
it already works. Add a native helper only when the fallback is absent or cannot
provide the required semantics.

Regular expressions need a specific compatibility decision. Python pattern,
match, flag, group, span, and substitution behavior cannot be assumed to match
Go's regular-expression package.

## Implementation order

Use these milestones as vertical slices rather than implementing whole modules
at once:

1. Decide the runtime-owned module and host-capability contracts for `sys`,
   `_io`, clocks, signals, and filesystem access.
2. Make unchanged `abc` import with tested metaclass and abstract-class behavior.
3. Implement in-memory `_io` and `io.StringIO`, then make exact
   `import unittest` succeed.
4. Run one passing and one failing `TestCase` through `TestResult`, including
   skip, cleanup, subtest, and traceback paths as separate slices.
5. Run `TestSuite` and `TestLoader` against already imported modules without
   filesystem discovery.
6. Run `TextTestRunner` against `StringIO` and compare its complete text output.
7. Replace the adapted colorsys test with unchanged CPython
   `test_colorsys.py`.
8. Add runtime-owned arguments and streams for `unittest.main()`.
9. Add capability-backed filesystem discovery and signal handling.
10. Plan `unittest.mock` and `IsolatedAsyncioTestCase` independently.

At every step, execute the unchanged pinned source. Keep the Go `testing`
package as the outer runner and create a fresh Bullsnake runtime for isolated
Python test modules. Python's `unittest` code should own test selection,
assertion behavior, results, and output once it can run.

## Completion checks

The synchronous in-memory milestone is complete when:

- the unchanged package imports from the vendored CPython 3.14.7 tree
- ordinary test methods, fixtures, cleanups, skips, expected failures, subtests,
  and assertion failures produce the expected `TestResult`
- exception formatting uses Python-visible traceback state
- `TestSuite` and name-based `TestLoader` execution work
- `TextTestRunner` writes the expected output to `StringIO`
- unchanged `test_colorsys.py` passes through the normal standard-library Go
  runner
- the full repository checks pass without network access or a Python executable

Discovery, catch-break signals, mock, and async support have their own completion
checks after this milestone.

## Current pause

Development is paused before `_io`, `sys`, filesystem, clocks, signals, threads,
and other host-facing native APIs. The next implementation should begin with an
architecture decision for runtime capabilities and runtime-owned module
factories. Weak-reference work also needs a memory and callback design before
`_weakref` is exposed.
