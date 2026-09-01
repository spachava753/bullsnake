# case: builtins module shares runtime namespace
import builtins
from builtins import abs as imported_abs

assert builtins.__name__ == 'builtins'
assert builtins.__package__ == ''
assert builtins.abs is abs
assert imported_abs is abs
assert builtins.sorted is sorted
assert builtins.NotImplemented is NotImplemented
setattr(builtins, 'temporary_builtin', 42)
assert temporary_builtin == 42
assert delattr(builtins, 'temporary_builtin') is None
try:
    temporary_builtin
except NameError:
    pass
else:
    assert False
