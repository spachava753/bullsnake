# First major milestone: run Python's unittest

Status: host configuration, synchronous in-memory I/O, ABCs, and substantial
collection behavior are implemented and tested. Importing and running `unittest`
is still blocked; no unittest test has executed.

The goal is to run CPython's unchanged synchronous `unittest` package, then use
it to run unchanged standard-library tests. The current work expands its
unchanged source dependencies and implements the runtime operations they need.
General weak references, Python-visible tracebacks, warning handling, and broader
introspection remain important gaps.

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

### Current progress and remaining work

This table is the current milestone tracker. Historical probe failures below
are retained as context, not as the current blocker. Update this table and the
[current unchanged-source checkpoint](#current-unchanged-source-checkpoint)
when a tested slice changes what can execute.

| Milestone | Current evidence | Remaining work |
| --- | --- | --- |
| Runtime and host groundwork | Source execution tests; per-runtime modules and arguments; supplied streams and performance counter; explicit host denial. | Broader object protocols, collection keys, introspection, and system-module APIs as dependencies require them. |
| Unchanged `abc` and `io` | Both import; ABC construction/registration and public io stream tests pass. In-memory streams, buffering, text decoding, and close/error paths are tested. | Full upstream conformance is not claimed; host-stream ABC integration and documented codec/buffer gaps remain. |
| Unchanged `_collections_abc` | Imports; structural protocols and Set/Mapping/Sequence families have behavior tests. Callable aliases support construction, call, equality/hash, TypeVar/ParamSpec specialization, defaults, and class bases. | Concrete Callable representation, ByteString warning behavior, forward references, Concatenate, TypeVarTuple unpacking/substitution, and broader alias forwarding. Import success is not completion. |
| Annotation and warning dependencies | Unchanged-source expansion selected on 2026-09-11; six initial dependencies are vendored at the pin. Code inspection, function globals/live closure cells, and the initial truthful `sys.implementation`/SimpleNamespace subset have execution tests. | Current entry blockers: `object.__init__` in types/enum, `_ast` in ast/annotationlib, and `_contextvars` in warnings. Implement real runtime operations rather than replacement Python APIs. |
| Unchanged `unittest` import | Currently stops at `unittest/result.py:5:8`, missing `traceback`. | Finish the active collections dependency work, then continue through traceback and the remaining synchronous import closure. |
| `TestCase` / `TestResult` | Not executed. | Passing tests, assertion failures, errors, setup/teardown, cleanups, skips, expected failures, and subtests. |
| Suites, loader, text runner | Not executed. | In-memory suite/name loading, traceback reports, warnings, and complete output checked through `StringIO`. |
| Unchanged colorsys tests and `unittest.main()` | Colorsys still uses adapted assertions. | Run the unchanged upstream test through unittest, then argument-driven entry with supplied streams and catchable exit. |

### Dependency strategy: unchanged Python sources

On 2026-09-11, we chose to expand the unchanged CPython dependency tree instead
of publishing limited native `annotationlib` or `warnings` replacements. This
applies to the remaining collections work as well as the unittest import path.
Keep every vendored module byte-for-byte at the pinned revision.

The immediate paths are:

- Concrete `Callable.__repr__` imports `annotationlib.type_repr`.
  `annotationlib` imports `ast`, `enum`, and `types`, among other modules, and
  performs real class/descriptor work while importing. Do not copy just
  `type_repr` into a substitute module or skip those imports.
- ByteString subclass construction and instance checks call
  `warnings._deprecated`. Python 3.14's `warnings` imports `_py_warnings` before
  optionally using `_warnings`. Test warning filtering, recording, emission,
  and error propagation rather than making `_deprecated` a no-op.
- Forward references and the remaining typing operations may share annotation
  dependencies. Follow the actual pinned source paths; do not assume that
  importing annotationlib finishes typing support.

Implement native machinery where Python genuinely depends on interpreter
services, such as `_ast`, frame/code metadata, descriptors, or synchronization.
Optional native accelerators may remain absent when the unchanged Python
fallback works. Neither a native helper nor an import-time declaration may
pretend to implement behavior it cannot execute. Warning output and any future
filesystem, clock, thread, or process interaction must preserve explicit host
capabilities; dependency expansion does not authorize ambient host access.

For each slice: add a source behavior test first, implement the missing operation,
record supported behavior and the next observed blocker here, run all repository
checks, and commit the finished slice. Record source provenance when vendoring.
Separate import evidence from exercised public behavior and from full upstream
conformance. Do not replace the adapted colorsys tests until unittest executes.

### Implemented and tested in this repository

| Area | Progress relevant to unittest |
| --- | --- |
| Python execution | Functions, all parameter kinds, decorators, closures, classes, inheritance, loops, comprehensions, generators, exceptions, and context managers have execution tests. |
| Objects and builtins | Method binding, properties, `super`, attribute helpers, type checks, user-defined iteration and comparisons, sorting, and many string and collection methods work within the documented subset. |
| Imports | Source modules, regular packages, relative imports, repeated and circular imports, and cleanup after a failed import are tested. |
| Go-backed modules | Each runtime starts with `builtins`, `__future__`, `_functools.cmp_to_key`, and `string.templatelib`, plus its `string` parent package. Private per-runtime Go constructors now initialize modules through the importer; `sys` exposes isolated arguments, borrowed UTF-8 streams, and catchable exit; `time.perf_counter` uses only the supplied counter. `_io` provides the documented synchronous in-memory stream subset and explicit filesystem denial; unchanged io imports, while unittest still has import dependencies. |
| Standard-library tests | Unchanged `colorsys.py` runs with adapted versions of all eight upstream public test methods. These use plain assertions, not `TestCase` objects. |

The [language tests](../internal/runtime/testdata/execution/) run Python source
through parsing, name resolution, compilation, bytecode validation, and
execution. The [standard-library runner](../stdlib/stdlib_test.go) uses the
filesystem loader and creates a fresh runtime for each test module.

The checked-in tree now also contains unchanged `operator`, `keyword`, and
`heapq` with passing source-to-result smoke tests. The synchronous unittest
sources and the initial io/abc dependency sources are vendored unchanged for
offline reproduction. The full dependency closure is not present, unittest still
does not import, and colorsys still runs adapted assertions.

### Results from the earlier compatibility probes

The last recorded sweep compiled all 61 Python source modules observed while a
pinned CPython process imported synchronous `unittest`. The parser, name
resolver, and compiler handled those files. This does not show that every
function in them can execute.

The earlier attempt, before `_io` was implemented, stopped at:

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
and again after the host slices, with the same import outcomes. Operator,
keyword, and heapq now also have checked-in execution regression tests. The
historical 61-module compilation sweep has not been repeated.

After the weak ABC registry slices, `abc` imported with checked-in execution
tests. After the first `_io.StringIO` output slice, `unittest` reached
`io.py:56:1` and failed because `_collections_abc` was missing. Both import
blockers are now resolved. The latest exact failure and reproduction command
are in the [current checkpoint](#current-unchanged-source-checkpoint).
No TestCase/TestResult, suite/loader, text runner, or unittest.main execution has
succeeded yet. An import success alone does not establish that a module's public
functions work.

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
The initial in-memory io/ABC implementation is described in the progress sections.
The Go names below illustrate the host design; the public API will follow tested
internal implementations.

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
structured exception arguments, including text that follows mutations of retained
native values. `sys.exit` raises through the ordinary VM path. Preserve relevant `args`, `errno`, and
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
provider failures, short writes, split UTF-8 input, Unicode counts and structured
codec exceptions (including subclasses and bare-raise validation), repeated
close, flush failures, and SystemExit. Streams remain non-seekable. They use
strict UTF-8 and fixed LF line boundaries without newline translation.

The host wrappers currently support direct reads/writes and context management;
iteration, writelines, and io ABC inheritance remain future slices. In-memory
StringIO now has the output behavior described below. `sys.modules` and active exception/traceback state are also still
missing. None of these host tests establishes unittest compatibility.

## What to do next

### 1. Finish collection behavior through unchanged dependencies

The initial module/capability, ABC, and in-memory I/O slices are implemented.
Continue with the [unchanged-source dependency strategy](#dependency-strategy-unchanged-python-sources):
vendor annotation and warning dependencies at the pin, reproduce their earliest
failure offline, and implement the required runtime primitives in tested slices.
Exercise concrete Callable representation and ByteString's actual warning hooks
through `_collections_abc`, not just standalone helper tests. Keep the remaining
alias and collection gaps in the progress table visible until tested.

### 2. Complete the synchronous unittest import closure

Once the active collections work is usable, continue from the current missing
`traceback` import through the remaining dependencies. Add only the sources and
runtime behavior reached by the selected synchronous path; mock, async runners,
filesystem discovery, and signal delivery remain outside this milestone.

Retain the existing StringIO and public io regression tests. Additional
`_colorize`, stream, exception-state, and warning requirements must use actual
runtime state and caller-supplied capabilities, not fabricated results.

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
ABC-style descriptor subclasses now construct and bind, including initialization
through `super` and abstractproperty markers. Subclassing `type`,
selecting `metaclass=`, and calling metaclass `__new__` now have source-to-result tests.

The first ABC allocation slice is tested: assigning `__abstractmethods__` to an
ordinary user class prevents instantiation when its truth value is true. The
runtime retains the metadata, isolates it from subclasses, supports deletion and
reassignment, and formats sorted missing-method diagnostics through Python
iterator/comparison continuations. Truth failures leave the prior state intact.
Runtime source fixtures test the native helpers, and a standard-library test
now exercises unchanged `abc.py`. Python weakref callbacks and regex compatibility
remain deferred. `_weakref` remains absent.

The selected route implements the native `_abc` helpers used by unchanged
`abc.py`. Class construction, abstract checks, virtual registration, and
metaclass instance/subclass checks now have source-to-result tests. `_get_dump`
supplies real weak diagnostic references, so unchanged `abc.py` uses this route.


### Abstract-method computation

The private Go `_abc` module now exposes `_abc_init` for the implemented
abstract-method computation subset. It snapshots direct class attributes,
resolves their live `__isabstractmethod__` markers, then iterates inherited names
and resolves overrides through ordinary class lookup. A successful computation
stores a frozen set and updates the allocation flag. Attribute, iterator, and
truth callbacks run in the VM; a failure leaves the previous abstract metadata
unchanged. Properties check getter, setter, and deleter markers in order;
classmethod and staticmethod markers follow their wrapped values. Bound methods
expose the underlying function's marker.

`_abc_init` now also installs fresh `_abc_impl` state. Virtual registration,
instance/subclass checks, cache tokens, and registry/cache reset helpers are
implemented. Registries and both caches hold Go weak pointers to class
allocations, with lazy pruning. Tokens are isolated per runtime; new registration
invalidates negative caches across ABCs. Checks honor subclass hooks before
nominal inheritance, then registered classes and immediate subclasses, through
ordinary VM continuations. Hooks must return bool or NotImplemented.

Source tests cover transitive registration, cycle rejection, native and exception
class registration, cache resets, mutations during callbacks, reported versus
actual instance classes, and errors. Go tests verify that actual ABC registries
and caches do not retain discarded classes and that runtime tokens are isolated.
General metaclass hashing/equality and native-base subclass enumeration remain
outside this subset.

`_get_dump` returns independent sets sharing callback-free weak class references.
They are callable, return None after collection, cache their target's hash, and
compare by live class identity; distinct dead references compare unequal. Saved
dumps do not retain their target classes. Their `__callback__` is None, unlike
CPython's private registry-removal callbacks: Bullsnake prunes on access. These
references have no public constructor and do not expose `_weakref` or `weakref`.
Python callbacks and regex compatibility remain deferred.

Unchanged `abc.py` now imports through the native helpers. The project-owned
standard-library regression test executes ABC/ABCMeta construction, modern and
legacy abstract decorators, concrete overrides, virtual and transitive
registration, structural hooks, instance checks, and cache resets. It is not
CPython's full test_abc suite. `update_abstractmethods` now runs unchanged after
method replacement and deletion, including explicit subclass recomputation.
`_dump_registry` now executes unchanged with explicit or redirected streams;
its complete report is checked against the runtime's weak-reference repr.

### Metaclass construction checkpoint

User classes may now inherit `type`. Class statements select the most-derived
compatible metaclass before executing the body, call `__prepare__`, and pass its
exact dictionary to `__new__` and `__init__`. Python factory functions are also
accepted as metaclasses. Dictionary namespaces retain body writes and deletions;
custom mapping namespaces remain unsupported. `type.__new__`, metaclass `super`,
class-cell propagation, inherited metaclass identity, and dynamic `type`
construction share the same class builder. Construction exceptions propagate
through the existing VM and are catchable at the class statement.

Native operations can retain ordered result continuations on their Python caller
while a child frame executes. They resume after the child's existing protocols
complete; error continuations run before the caller's Python exception handlers.
This supports metaclass call sequences without using the Go stack for Python
calls. Generic instance `__new__`, custom metaclass `__call__`, `__init_subclass__`,
and general metaclass descriptor precedence remain separate gaps. The tested ABC
subset and remaining API gaps are described above.

### Internal weak class references and deferred Python callbacks

A weak reference lets code refer to an object without keeping it alive.
`unittest.signals` creates a `WeakKeyDictionary` even when Ctrl-C handling is
unused, so choosing `_abc` does not remove the package's weak-reference need.

A dictionary that holds strong references would keep test results alive and
would not reproduce weak-reference callbacks. Bullsnake uses Go's garbage
collector. Any Python callback triggered by collection must wait until the
interpreter can safely run it, rather than running inside a Go cleanup callback.
The internal lifetime choice is now Go `weak.Pointer` to actual class allocations,
with dead entries pruned synchronously on access. Immediate user-subclass links
use this storage and have source behavior plus Go GC ownership tests. ABC
registries and caches use the same approach. Public `_weakref` and callback delivery
remain deferred; no second collector or CPython reference counting is planned.

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
- Broader native-type subclass construction, especially container subclasses
  needed by code such as `namedtuple`. Initial descriptor subclasses now work.
- Dictionary and set keys whose hashing or equality calls Python methods.
  Direct `hash(obj)` supports a user method today, but container keys do not.
- Set and frozen-set equality involving user-defined element equality.
  These containers still use fixed runtime comparisons; list, tuple, and
  dictionary equality now resume Python equality and truth callbacks.
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
Plan mock and async testing separately. The independent ABC class-construction
and abstract-method computation slices are tested. Virtual registration is tested
through both native helpers and unchanged `abc.py`, which now imports. In-memory
`_io` now has tested in-memory streams, buffering, text decoding, and explicit
filesystem denial. Unchanged `io.py` now imports and has public stream ABC,
Reader/Writer protocol, subclass, and integrated stream regression tests.
Collection ABC behavior tests now exercise native and user structural checks for
Iterable, Iterator, Sized, Container, and Collection, including disabled slots
and concrete ABC defaults. Collection mixin tests also cover Set/MutableSet comparison, forward/reflected
operators, in-place mutation, remove/pop/clear, and matching hashes between
custom immutable sets and native frozensets. Mapping/MutableMapping tests cover
live KeysView/ItemsView/ValuesView objects, view set operations and membership,
lookup/defaults, updates from mappings and iterable pairs, removal/clearing, and
equality including Python value callbacks. View representation now executes the
unchanged numbered-field format string through real attribute and repr callbacks.
Sequence/MutableSequence tests exercise iteration, containment/index callbacks,
reverse iteration, append/extend/pop/remove/clear/reverse, and in-place extension.
Sequence.count now uses sum with integer accumulation and Python comparison
callbacks. Hashable and Callable checks now cover native scalars, containers, functions,
methods, and classes as well as user slots and explicit None overrides. The coroutine/generator and async-generator ABC defaults now have unchanged-source
execution tests for delegation, throw/close completion and errors, structural
hooks, abstract enforcement, and native registrations. They use the existing
coroutine/await VM path, not an event loop. Buffer structural tests now use native
bytes/bytearray/memoryview descriptors with real exports, flags, ownership, and
release semantics. GenericAlias subclasses now run __new__/__init__ and native
state construction; unchanged Callable aliases flatten argument lists, preserve
origins, and round-trip reduction tuples. Alias comparison and hashing resume
Python callbacks; parameter discovery caches identity-ordered metadata. Ordinary
TypeVar and ParamSpec specialization, lazy defaults, nested substitutions, and
class-statement base rewriting now have execution tests, including unchanged
Callable specialization and Sequence/Mapping/Callable alias bases. ByteString
warnings and concrete Callable representation still require their warning and
annotation dependencies. String forward references, Concatenate, TypeVarTuple
unpacking/substitution, and broader alias forwarding remain unfinished.
No unittest test has executed yet; the overall milestone remains blocked.

### Printing to Python streams

`print` accepts arbitrary positional values and keyword-only `sep`, `end`,
`file`, and `flush`. It converts flush truth first, resolves the current
`sys.stdout` when file is omitted or None, and retains that stream for the call.
Each write method is resolved before converting its value through Python str.
Separators, values, and the ending are written separately; earlier output is
preserved if conversion, writing, or flushing fails. Return values from stream
methods are ignored. Python callbacks use VM continuations, and host wrappers
retain their existing Unicode, error, and borrowed-ownership contracts.

Under Bullsnake's explicit host policy, a None stdout raises PermissionError
instead of CPython's disconnected-stdout no-op. There is no ambient output or
silent sink. A deleted sys.stdout raises RuntimeError. Explicit Python streams
work without host output providers. print currently inherits the existing str
limitations, including representations of containers holding user objects.


### Implemented in-memory `_io` scope

The `_io` stream implementation now includes:

- StringIO and BytesIO, with text/byte positions, reads, writes, lines, snapshots,
  truncation, close, reinitialization, subclass behavior, and BytesIO exports.
- IOBase, RawIOBase, BufferedIOBase, and TextIOBase defaults and Python callback
  delegation, including scoped writable-buffer leases.
- BufferedReader, BufferedWriter, BufferedRandom, and BufferedRWPair, with
  short/nonblocking I/O, partial-write retry counts, cursor synchronization,
  explicit lifecycle, and failure recovery.
- IncrementalNewlineDecoder, including Python codec delegation and pending CR.
- TextIOWrapper with UTF-8, ASCII, and Latin-1, incremental decoding, newline
  translation, output buffering, character limits, position round trips,
  truncation, reconfiguration, and close after flush failure.
- text_encoding and argument-checked open/open_code/FileIO denial boundaries.
  No path, descriptor, opener, locale request, or source loader grants ambient
  host access.

Behavior runs through the parser, compiler, validator, and VM in Python source
fixtures with Go as the outer runner. Integrated tests print Unicode through
TextIOWrapper and BufferedRandom to BytesIO, then decode it, restore positions,
and close the stack. Other tests exercise partial operations, EINTR without
duplicate output, index callbacks, nonblocking results, exported views, readonly
and strided buffers, and failure cleanup. Go-facing tests verify runtime
identity, provider ownership, GC lifetime, and cleanup after loader errors.
Follow-up text regressions cover malformed UTF-8 prefix boundaries, recovery
after decode failures without replaying partial output, subclass readline
iteration, and reconfiguration after decoded buffers are cleared. Expected
behavior is checked against the pinned CPython decoder and TextIOWrapper source.

The initial buffer prerequisite includes bytearray and one-dimensional unsigned
byte memoryviews. Views strongly retain exporters; weak export tables do not
retain the views. Explicit release, frame leases, and buffered-operation guards
provide deterministic operation cleanup. Python is never called from Go GC.

The implementation remains a selected CPython subset. TextIOWrapper defaults to
UTF-8, denies locale access, and supports strict/ignore/replace errors for the
three implemented stateless codecs. Restore positions are local byte offsets,
not CPython's opaque integer-cookie layout. Stateful codecs, codec registration,
multidimensional views, arbitrary Python buffer exporters, pickle state methods,
and comprehensive I/O introspection are not implemented. Instance `__dict__`,
general weakref callbacks, and regex compatibility remain deferred.

### Current unchanged-source checkpoint

With class namespace views and real frame-local mappings implemented,
unchanged `_collections_abc.py` and `io.py` now import successfully. Reproduce:

```sh
go run ./tools/importprobe stdlib/3.14 abc _collections_abc io unittest
```

The first three modules import. Unittest advances to:

```text
stdlib/3.14/unittest/result.py:5:8: ModuleNotFoundError: No module named 'traceback'
```

This is an execution probe, not a unittest success claim. The public io ABCs now
have regression tests for registration, Reader/Writer structural checks and
abstract enforcement, subclassing, and integrated streams. Collection mixin
families and generic-alias construction, equality/hash, TypeVar/ParamSpec
specialization, and class bases now have the behavior tests described above.
This is still not the full collections or alias surface. No unittest TestCase, suite, runner report,
unchanged test_colorsys.py, or unittest.main() has executed yet. The complete
synchronous unittest milestone remains unfinished.

#### Annotation and warning dependency probes

The first dependency batch was copied unchanged from the same CPython pin:
`annotationlib.py`, `ast.py`, `enum.py`, `types.py`, `warnings.py`, and
`_py_warnings.py`. Reproduce their current import failures offline:

```sh
go run ./tools/importprobe stdlib/3.14 types enum ast annotationlib warnings
```

| Probe | First observed failure |
| --- | --- |
| `types`, `enum` | `types.py:50:34`: object has no `__init__` attribute. |
| `ast`, `annotationlib` | `ast.py:23:1`: missing `_ast`. |
| `warnings` | `_py_warnings.py:4:8`: missing `_contextvars`. |

The first types blocker, function `__code__`, is resolved. Function `__code__`
and frame `f_code` share runtime-owned immutable code wrappers with names,
source location, argument counts, local/cell/free names, and the implemented
execution flags. Source tests cover real functions, closures, and suspended
function kinds; Go tests cover wrapper identity across runtimes. Code mutation,
construction, CPython instruction bytes, and complete code/line metadata remain
unsupported.

The next types blocker, `sys.implementation`, is also resolved for the initial
metadata subset: `name='bullsnake'` and `cache_tag=None`. Its runtime-owned
SimpleNamespace type supports real construction/reinitialization, keyword and
current dict/iterable-pair input, native/Python subclass initialization, and a live
writable attribute dictionary. Implementation version/platform fields are not
invented. Namespace comparison, representation, reduce/replace, and custom
mapping construction still need later slices.

Function `__closure__` now exposes the actual captured cells, and `__globals__`
returns the live module dictionary. Cell contents support reading, writing,
deletion, and empty-cell errors through the same lexical storage used by nonlocal
variables and frame-locals proxies. Tests retain closures after return and check
that every mutation path observes the same binding. Cell constructors/comparison
and function construction remain unfinished.

The table lists first failures, not complete missing-feature lists. Next,
implement the actual native object initialization/string descriptors needed by
types, rather than publishing marker descriptors for type discovery. Subsequent
types import work includes native method/attribute descriptor identities,
traceback/frame objects, and union types. AST services, context-variable state,
enum construction, annotation descriptors, and subsequent imports need their own
tested slices. Vendoring source alone is not a passing behavior test.
