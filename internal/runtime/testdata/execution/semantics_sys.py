# name: default arguments and absent streams
import sys
assert sys.argv == [""]
assert sys.stdin is None
assert sys.stdout is None
assert sys.stderr is None
assert sys.__stdin__ is sys.stdin
assert sys.__stdout__ is sys.stdout
assert sys.__stderr__ is sys.stderr
sys.stdout = "replacement"
import sys as again
assert again is sys
assert again.stdout == "replacement"
assert sys.__stdout__ is None
