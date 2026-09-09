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
| Go-backed modules | Each runtime starts with `builtins`, `__future__`, `_functools.cmp_to_key`, and `string.templatelib`, plus its `string` parent package. Private per-runtime Go constructors now initialize modules through the importer; `sys` exposes isolated arguments, borrowed UTF-8 streams, and catchable exit; `time.perf_counter` uses only the supplied counter. `_io` and the remaining import dependencies are still missing. |
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
The five import probes were repeated against main at `d202a6f` on 2026-09-09
and produced the same results. The historical 61-module compilation sweep has
not been repeated. Working imports still need checked-in regression tests. An import success alone does
not establish that a module's public functions work.

An execution smoke test after the host slices exposed missing list item
assignment inside `heapq.heapify`. Native integer/boolean list assignment now has
source fixtures; slice mutation remains unsupported.

The evidence supports moving on to system-module work. It does not establish
that all the language features needed by `unittest` are finished.

## Design for Go-backed modules

The chosen direction is to describe host capabilities with small, composable Go
interfaces, following the approach used by `io/fs`. A capability is an operation
the host makes available, such as writing output or reading a clock. The caller
can supply any implementation that satisfies its interface.

The private constructor registry and initial `runtime.Config` argument/loader
configuration are implemented and tested. The performance counter is implemented with fake-provider and denial tests.
Borrowed UTF-8 stream adapters now have source fixtures and Go provider tests.
The broader io/ABC surface below remains planned. The Go names below are illustrative; the public API
will follow tested internal implementations.

### Small interfaces, supplied explicitly

Go's `fs.FS` requires only `Open`. Helpers such as `fs.Stat` and `fs.ReadDir` use
optional interfaces when available and otherwise try the operations on an opened
file. Follow that pattern: require only what an operation needs, and use extra
interfaces for additional behavior or a correct fallback.

Reuse standard Go interfaces where their meaning fits. The first configuration
needs `io.Writer` for output and `io.Reader` for input. Read-only filesystem
access can use `fs.FS` later, once its path rules are defined. Add a Bullsnake
interface only when the standard library has no suitable contract.

For example, the initial configuration could look like this:

```go
// Proposed types, not an implemented API.
type Flusher interface {
    Flush() error
}

type Terminal interface {
    IsTerminal() bool
}

type PerfCounter interface {
    PerfCounter() time.Duration
}

type HostConfig struct {
    Args    []string
    Stdin   io.Reader
    Stdout  io.Writer
    Stderr  io.Writer
    Counter PerfCounter
}
```

Output can go to a `bytes.Buffer`; a custom writer needs only `Write`, without
also implementing input, seeking, or terminal operations. A `bufio.Writer`
already implements `Flusher`. Tests can supply a predictable counter.
Applications can supply their own implementations without depending on Python
object types. Filesystem access is absent from this first configuration; the
existing source loader remains a separate way to load Python files.

Use typed configuration fields, not an untyped collection of services or one
large interface with a method for every system operation. Each adapter must name
the optional interfaces it recognizes. Check those interfaces on the supplied
value, rather than checking concrete Go types or exposing every available method.

The initial output adapter recognizes `Flusher` and `Terminal`. It adds no
buffering of its own. Without `Flusher`, `flush` has nothing to drain; without
`Terminal`, `isatty` returns false. A provider with its own pending buffer must
provide `Flush` if Python is to flush it. An implemented method can still fail;
its presence does not guarantee success for every underlying resource.

Keep the initial host text streams non-seekable, even if the supplied object
implements `io.Seeker`. A Go file representing a pipe still has a `Seek` method,
and Go byte offsets do not implement Python text-stream positions. Those
positions can include decoding state. `StringIO` owns its separate in-memory
position rules. A later file adapter can add seeking with the appropriate
Python behavior and tests.

The counter returns a nondecreasing duration from a fixed, arbitrary origin.
The Python wrapper converts it to seconds for `time.perf_counter()`. It grants
no wall-clock, sleeping, timer, or scheduling operations. Define those interfaces
separately if later tests need them.

### Python behavior stays in the module implementation

The caller provides Go services. Bullsnake's Go-backed module code turns them
into Python values and implements argument checks, return values, exceptions,
and object behavior. It must use the same call and attribute paths as other
runtime values.

| Python module or behavior | Source of its state or operations |
| --- | --- |
| `sys.argv` | Copy configured arguments into a fresh Python list. If the configuration supplies no entries, use `[""]`; unchanged `unittest.main()` reads `argv[0]`. |
| Platform information | Describe the configured environment and Bullsnake implementation without silently copying process globals or pretending to be CPython. |
| `sys.stdin`, `stdout`, and `stderr` | Python stream wrappers around the supplied Go reader or writers, or `None` when absent. Python can replace these attributes without changing the original configuration. |
| `sys.__stdin__`, `__stdout__`, and `__stderr__` | Initially reference the same objects as the corresponding standard streams. Replacing `sys.stdout`, for example, leaves `sys.__stdout__` unchanged. |
| `sys.modules`, `exc_info`, and `exception` | The runtime's live module cache and active handled exception. These are interpreter state, not host-provider interfaces. |
| `sys.exit` | Raise Python `SystemExit`. Python may catch it; an uncaught exit crosses the Go execution boundary as a Python exception. Never call `os.Exit`. |
| `_io` in-memory streams | Runtime-owned objects with no filesystem capability. `StringIO` implements Python text positions, character counts, and close behavior. |
| `_io` file streams and `os` filesystem operations | Explicit filesystem access, with additional interfaces for operations beyond read-only access. |
| `time.perf_counter` | The configured counter only. |
| `signal` | Runtime-owned Python handlers plus separately supplied host signal access, when signal handling is implemented. Importing the module must not install process handlers. |

The text wrapper must not confuse Go byte counts with Python character counts.
For the first host-stream adapter, use a documented UTF-8 encoding policy and
test non-ASCII text, partial writes, and errors. Other encodings and configurable
newline handling can follow. Do not claim that `io.Writer` itself implements
Python's text-stream contract.

Operations that use `sys.stdout` or `sys.stderr` must resolve the current Python
attribute, rather than bypassing it and writing to the configured Go writer.
An API given an explicit stream, such as `TextTestRunner`, may retain that stream
as Python specifies. Importing `sys` again returns the same module and preserves
any replacements.

### Missing capabilities do not grant host access

Configuration supplies authority as well as an implementation. No filesystem,
clock, stream, environment, or signal operation may fall back to ambient Go
process state. An eventual convenience configuration that uses the real host
must be an explicit choice.

Keep module availability separate from permission to perform an operation.
`sys`, in-memory `_io`, and pure path or constant helpers should work without
filesystem access. Imports still have to satisfy the real Python dependencies;
this rule does not permit placeholder classes or functions.

An unconfigured standard stream appears as `None` in both its ordinary and
original `sys` attributes. Callers that need output must supply a stream;
Bullsnake does not quietly use `os.Stdout` or discard the output.

Keep configuration errors, denied access, and operation failures distinct:

| Situation | Result |
| --- | --- |
| Invalid Go configuration, such as duplicate module registrations | Return a Go error during runtime construction, before Python runs. Omitting an optional capability is valid configuration. |
| Host capability not supplied | Raise `PermissionError` when the operation is attempted. This includes a missing counter and, later, missing filesystem access. Never substitute fabricated results or consult the host process. |
| Operation not supported by a particular Python stream | Raise `io.UnsupportedOperation`. The initial host text adapters reject seeking even when their Go provider has a `Seek` method. |
| Go provider reports an I/O failure | Translate it into the appropriate `OSError` subclass at the Python module boundary. |
| Reading or writing a closed Python stream | Raise `ValueError`. Repeated `close` calls have no effect; other methods follow their Python closed-stream rules. |

The missing-capability rule is Bullsnake's explicit host-access policy.
`NotImplementedError` must not stand in for intentionally withheld access.
Unimplemented interpreter behavior keeps its existing rejection checks until
implemented; ordinary Python operation failures are not invalid bytecode.

Error support is a prerequisite for the adapters. The runtime now implements
`SystemExit` and its `code` attribute, the relevant `OSError` subclasses, and
structured exception arguments. `sys.exit` raises through the ordinary VM path. Preserve relevant `args`, `errno`, and
Python-visible filenames rather than reducing provider errors to strings.
`io.UnsupportedOperation` must match both `OSError` and `ValueError`; the current
additional-base mechanism used by exception groups offers a starting point.

Use `errors.Is` for classifications such as `fs.ErrNotExist` and
`fs.ErrPermission`, and `errors.As` to inspect structured errors such as
`fs.PathError`. Report paths as seen by Python, not incidental host paths used
inside an adapter. Keep end of input separate from failure, and retain bytes
returned alongside an error before handling that error.

A fallback may only use already supplied capabilities. Read-only file access
must not discover a concrete `*os.File` and acquire unrelated process access
through its descriptor. Likewise, a missing optimized filesystem method must
not trigger a call to Go's process-wide `os` functions.

### Filesystem paths and resource ownership need their own rules

Use `fs.FS` later for the read-only operations it can represent, with
`fstest.MapFS` as one useful test provider. It does not define file creation,
writes, renames, removal, or Python's complete path behavior. Add small
interfaces for those operations when discovery or file tests require them,
rather than requiring a full filesystem implementation upfront.

`fs.FS` names are slash-separated paths relative to a root, not arbitrary Python
paths. The Python adapter must define how its configured roots and working
directory map to those names, including absolute paths and `..`. Settle and test
that mapping before exposing file operations. An `fs.FS` value alone is not a
security sandbox; confinement, including symlink behavior, depends on the
provider. Importing Python source and granting Python file access are separate
choices, even if the host deliberately supplies the same filesystem to both.

Configuration copies ordinary data such as arguments but retains references to
capability providers. Providers passed in as standard streams are borrowed.
Closing a Python wrapper closes that wrapper and flushes as appropriate; it does
not close the host object merely because it also implements `io.Closer`.
A file opened for Python is owned by that Python stream and closes its returned
handle. Do not depend on garbage collection to close files.

Each runtime gets fresh modules, Python stream wrappers, and other mutable
Python state. The caller may deliberately share a Go writer or filesystem
between runtimes, but must then account for its shared state and concurrent use.

Provider calls are synchronous in the first implementation. An arbitrary
`io.Reader` or `io.Writer` may block indefinitely; passing a `context.Context`
through the interpreter cannot interrupt such a call by itself. Cancellable I/O
needs a separate provider contract later. The first API promises neither I/O
timeouts nor forced shutdown of a blocked provider.

Provider goroutines must not mutate Python objects or call Python directly.
Future signal delivery must hand events back to the interpreter for execution.

### Create Go-backed modules through the ordinary importer

Keep an internal registry of module constructors on each runtime. On import,
check the module cache first, then a registered Go constructor, then the source
loader. Reject duplicate Go registrations during configuration. Registered Go
modules take precedence over same-named source files; replacing a host provider
should not require replacing Python module source.

The importer creates and caches a module before initializing it, so repeated
and circular imports observe the same object. A Go constructor populates that
module for its runtime. Failed initialization follows the existing cache cleanup
rules, and package children use the existing parent binding rules. Initialization
must not open files or install signal handlers simply because a module was
imported.

The current `ModuleSpec` carries compiled Python code. Extend the internal
loading path to distinguish source execution from Go initialization; do not
invent empty bytecode to represent a Go module. Python code still receives full
bytecode validation, and any Python calls made by native operations still run
through the existing interpreter frame loop.

Supplying a capability must not require writing a module constructor. The first
milestone can keep constructors private while accepting caller implementations
of the small Go interfaces. A general public API for registering custom Python
modules remains separate work.

### Build and test the smallest useful configuration

First implement module creation and isolated configuration. Add the exception
types and attributes needed by each operation before exposing its stream or
counter adapter. Then add the streams, active exception state, and counter
needed by the in-memory test runner. Keep Python filesystem access, process
operations, and signal-handler control out of that first configuration. The
abstract-class, weak-reference, and traceback work below is still required;
Go interfaces do not remove those Python compatibility requirements.

Add behavior tests that prove:

- Empty argument configuration produces `sys.argv == [""]`, and configured
  argument lists are copied rather than shared between runtimes.
- Missing streams initialize both ordinary and original `sys` attributes to
  `None`. Supplied streams initially share identity with their original
  attributes, and later replacements leave those originals unchanged.
- A minimal writer works without optional interfaces. Output wrappers use
  `Flush` and `IsTerminal` when supplied, but remain non-seekable even when the
  provider implements `io.Seeker`.
- Repeated imports preserve module identity, failed imports clean up correctly,
  and separate runtimes do not share mutable Python module state.
- Replacing a Python stream redirects operations that use that attribute.
  Closing a wrapper does not close a borrowed host writer, repeated close is
  harmless, and closed-stream operations and flush failures follow Python rules.
- Missing capabilities raise the documented errors without touching host
  resources. I/O exceptions retain their relevant attributes, and
  `io.UnsupportedOperation` is caught as both `OSError` and `ValueError`.
- Readers retain data returned alongside end of input or an error. Partial
  writes and write failures are not reported as complete successful writes.
  Text adapters handle non-ASCII text without confusing bytes and characters.
- A fake counter controls runner timing without consulting the real clock.
  A missing counter raises `PermissionError`.
- `SystemExit` preserves its `code`, can be caught in Python, and crosses the Go
  boundary as a Python exception when uncaught, without terminating the process.
- Unchanged Python dependencies and tests use these wrappers through the normal
  importer and interpreter.

### Tested host checkpoint

The internal configuration now supplies arguments, separate input/output/error
providers, and a performance counter. Tests cover defaults, original references,
runtime isolation, borrowed ownership, optional Flush/IsTerminal, denied timing,
provider failures, short writes, split UTF-8 input, Unicode counts, repeated
close, flush failures, and SystemExit. Streams remain non-seekable. They use
strict UTF-8 and fixed LF line boundaries without newline translation.

The host wrappers currently support direct reads/writes and context management;
iteration, writelines, io ABC inheritance, and the in-memory StringIO type remain
future slices. `sys.modules` and active exception/traceback state are also still
missing. None of these host tests establishes unittest compatibility.

## What to do next

### 1. Implement the module and capability design

Begin with the internal module constructors and typed configuration described
above. Implement each adapter's exception prerequisites before exposing its
operations. Prove isolation, defaults, denied access, and caller-supplied output
with focused tests before adding more system operations. Keep exact public Go
names open until those internal contracts work.

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
Plan mock and async testing separately. The immediate next task is `abc` class construction, followed by `_io` and
unchanged `io.py`. No unittest test has executed yet. The overall milestone remains blocked.
