# case: globals exposes the actual module dictionary through every frame kind
import sys
namespace = globals()
assert namespace is globals()
assert namespace is sys._getframe().f_globals
value = 1
assert namespace['value'] == 1
namespace['value'] = 2
assert value == 2
def read():
    local = 3
    assert 'local' not in globals()
    assert globals() is namespace
    return value
assert read.__globals__ is namespace
assert read() == 2
class Sample:
    local = 4
    assert globals() is namespace
    assert 'local' not in globals()
assert [globals() for item in range(2)] == [namespace, namespace]
def generator():
    yield globals()
assert next(generator()) is namespace
namespace['inserted'] = 5
assert inserted == 5
del namespace['inserted']
try:
    inserted
    assert False
except NameError:
    pass

# ---
# case: globals rejects arguments
try:
    globals(1)
    assert False
except TypeError as error:
    assert str(error) == 'globals() takes no arguments (1 given)'
try:
    globals(value=1)
    assert False
except TypeError as error:
    assert str(error) == 'globals() takes no keyword arguments'
