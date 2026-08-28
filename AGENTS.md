# Bullsnake project notes

Bullsnake is a Python interpreter written in Go. It supports a useful subset of
Python and aims to run more pure Python packages over time.

Before making changes, use these files as the sources of truth:

- `README.md` gives the short project overview.
- `docs/architecture.md` records project goals, design choices, and compatibility
  boundaries.
- `docs/impl.md` describes what the code does now and what is still missing.
- The code and tests define the current behavior. If they disagree with the
  documentation, resolve the difference as part of the change.

Read any more specific `AGENTS.md` file in the area being changed. Those files
may contain short-lived working rules and links to the relevant code or docs.
