# case: buffered readinto delegates to read and fills only returned bytes
from _io import _BufferedIOBase, _IOBase
assert issubclass(_BufferedIOBase, _IOBase)
class Reader(_BufferedIOBase):
    def read(self, size):
        assert size == 4
        return b'ab'
    def read1(self, size):
        assert size == 2
        return b'XY'
b = bytearray(b'....')
r = Reader()
assert r.readinto(b) == 2
assert b == b'ab..'
with memoryview(b) as view:
    child = view[1:3]
    assert r.readinto1(child) == 2
    assert b == b'aXY.'
    child.release()
# ---
# case: readinto pins bytearray and view until callbacks finish
from _io import _BufferedIOBase
b = bytearray(4)
v = memoryview(b)
class Reader(_BufferedIOBase):
    def read(self, size):
        try:
            b[:] = b'longer'
            assert False
        except BufferError:
            pass
        try:
            v.release()
            assert False
        except BufferError:
            pass
        return b'ok'
assert Reader().readinto(v) == 2
assert bytes(v) == b'ok\0\0'
v.release()
b[:] = b'longer'
assert b == b'longer'
# ---
# case: failed callbacks release buffer pins without partial copies
from _io import _BufferedIOBase
b = bytearray(b'kept')
class Failure(_BufferedIOBase):
    def read(self, size):
        raise LookupError('read')
try:
    Failure().readinto(b)
    assert False
except LookupError:
    pass
assert b == b'kept'
b[:] = b'resizable'
class Invalid(_BufferedIOBase):
    def read(self, size):
        return b'too much'
try:
    Invalid().readinto(bytearray(1))
    assert False
except ValueError:
    pass
for buffer in (b'readonly', memoryview(b'readonly'), memoryview(bytearray(4))[::2]):
    try:
        Failure().readinto(buffer)
        assert False
    except (TypeError, BufferError):
        pass
