import sys
s = sys.stdin
assert s is sys.__stdin__
assert s.readable()
assert s.flush() is None
assert not s.writable()
assert not s.seekable()
assert s.read(0) == ''
assert s.read(2) == 'hé'
assert s.readline() == '🙂\r\n'
assert s.readline(2) == 'ne'
assert s.read() == 'xt'
assert s.read() == ''
assert s.close() is None
assert s.close() is None
try:
    s.read(0)
    assert False
except ValueError:
    pass
