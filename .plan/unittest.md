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
