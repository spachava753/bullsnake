import sys
s = sys.stdout
try:
    s.flush()
    assert False
except PermissionError:
    pass
assert not s.closed
try:
    s.close()
    assert False
except PermissionError:
    pass
assert s.closed
assert s.close() is None
