# Synchronous unittest implementation

Base: d202a6f576e43e6cc435f16e9f2f8f482b9d89bc (main on 2026-09-09).
Feature branch: runtime/unittest-host-capabilities. Separate earlier dirty checkout left untouched.
Go: pinned minimum 1.27.0, available toolchain 1.27.1; pinned laas and x/text unchanged.
Reference: /workspace/scratch/183ed39df699/cpython-reference at 823f0323ee6ec1402088b73bce1a38473cac36dc.

Reproduction: go run ./.plan/probe /workspace/scratch/183ed39df699/cpython-reference/Lib
operator, keyword, heapq import successfully; abc fails at _weakrefset.py:5:1 (missing _weakref);
unittest fails at io.py:53:8 (missing _io). The historical 61-module compilation sweep has not been reproduced.
Baseline go test ./... passes.

Slices:
1. Private native constructor registry, ordinary importer identity and rollback; zero-authority configuration and sys arguments/defaults.
2. Structured exceptions, SystemExit, OSError family; sys.exit and performance counter.
3. Borrowed UTF-8 standard stream adapters, optional Flush/IsTerminal, failures and close.
4. Unchanged dependency imports: ABC/metaclass support, _io/io; weak-reference design before exposure.
5. Python tracebacks, test cases/results, suites/loaders, reports, colorsys, main.

Run all four required gates before each finished commit. Do not claim milestone completion from imports.

## Tested checkpoint

- 44901dc: private native constructors and isolated arguments/default sys attributes.
- 7b996b6: structured exception args, SystemExit/sys.exit, OSError family.
- 0941aab: caller-supplied performance counter; no ambient timing.
- ce4c3f6: borrowed strict UTF-8 standard streams, provider errors and Unicode errors.
- 70767d4: native integer/boolean list assignment, exposed by heapq execution.

The next commit vendors unchanged synchronous unittest and initial io/abc sources,
plus operator/keyword/heapq and their passing behavior smoke test. Every copied
source was compared byte-for-byte with its pinned Git blob, not just a checkout.

Offline reproduction:
    go run ./.plan/probe stdlib/3.14 abc unittest
Current failing operations:
    abc -> _weakrefset.py:5:1 -> from _weakref import ref -> ModuleNotFoundError
    unittest -> io.py:53:8 -> import _io -> ModuleNotFoundError

No unittest case, result, suite, loader, runner, or main has executed. Next work:
ABC/metaclass construction; _io and unchanged io; genuine weak references with a
settled lifetime/safe-VM callback design; Python traceback/frame/exception state;
remaining unchanged import dependencies and execution operations. Full source
closure and unchanged colorsys test replacement remain pending. No weak-reference
lifetime policy has been chosen or exposed; no strong-reference substitute exists.
Regex compatibility, filesystem path permissions, signals, and public extension
APIs remain separate decisions; no new authority was granted for them.

All four required gates passed before each finished commit. Final offline gates
use GOPROXY=off and GOSUMDB=off with Go 1.27.1 and the unchanged pinned tool/deps.
The historical 61-source compilation sweep was not repeated and is not new evidence.
