# case: integer native allocation shares numeric construction
assert int() == 0
assert int.__new__(int) == 0
assert int.__new__(int, 42) == 42
assert int.__new__(int, 4.9) == 4
assert int.__new__(int, True) == 1
assert int.__new__.__self__ is int
assert dict.__new__.__self__ is dict
assert object.__new__.__self__ is object
assert type(None).__new__.__self__ is type(None)
assert (42).__new__ is int.__new__
try:
    int.__new__(str)
    assert False
except TypeError as error:
    assert str(error) == 'int.__new__(str): str is not a subtype of int'
try:
    int.__new__(bool)
    assert False
except TypeError as error:
    assert str(error) == 'int.__new__(bool) is not safe, use bool.__new__()'
