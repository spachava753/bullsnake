# case: BytesIO reads writes and seeks use byte positions
from _io import BytesIO, _BufferedIOBase
assert issubclass(BytesIO, _BufferedIOBase)
s = BytesIO(b'abc\ndef')
assert s.tell() == 0
assert s.read(2) == b'ab'
assert s.readline() == b'c\n'
assert s.read1(None) == b'def'
assert s.seek(-2, 2) == 5
assert s.write(bytearray(b'XY')) == 2
assert s.getvalue() == b'abc\ndXY'
assert s.seek(-100, 1) == 0
assert s.seek(10) == 10
assert s.readinto(bytearray(3)) == 0
assert s.tell() == 10
assert s.write(b'!') == 1
assert s.getvalue() == b'abc\ndXY\0\0\0!'
assert s.truncate(4) == 4
assert s.tell() == 11
assert s.truncate(20) == 20
assert s.getvalue() == b'abc\n'
assert s.seek(0) == 0
b = bytearray(6)
assert s.readinto(b) == 4
assert b == b'abc\n\0\0'
# ---
# case: BytesIO native line operations ignore readline and write overrides
from _io import BytesIO
class Buffer(BytesIO):
    def readline(self, size=-1):
        raise AssertionError('native iteration should not call this')
    def write(self, value):
        raise AssertionError('native writelines should not call this')
s = Buffer(b'a\nb\nc')
assert s.readlines(2) == [b'a\n']
assert list(s) == [b'b\n', b'c']
assert s.writelines([b'X', b'Y']) is None
assert s.getvalue() == b'a\nb\ncXY'
# ---
# case: exported BytesIO storage prevents writes truncation and close
from _io import BytesIO
s = BytesIO(b'abc')
snapshot = s.getvalue()
v = s.getbuffer()
v[1] = 90
assert s.getvalue() == b'aZc'
assert snapshot == b'abc'
for call in (lambda: s.write(b''), lambda: s.write(b'x'),
             lambda: s.truncate(3), s.close):
    try:
        call()
        assert False
    except BufferError:
        pass
assert not s.closed
child = v[:]
v.release()
try:
    s.close()
    assert False
except BufferError:
    pass
child.release()
assert s.close() is None
assert s.close() is None
# ---
# case: BytesIO reinitialization follows native reset ordering
from _io import BytesIO
s = BytesIO(initial_bytes=b'abc')
assert s.getvalue() == b'abc'
v = s.getbuffer()
try:
    BytesIO.__init__(s, b'new')
    assert False
except BufferError:
    pass
assert s.tell() == 0 and s.getvalue() == b''
assert bytes(v) == b'abc'
v.release()
BytesIO.__init__(s, b'new')
assert s.getvalue() == b'new'
s.close()
BytesIO.__init__(s, None)
assert s.closed
BytesIO.__init__(s, b'reopened')
assert not s.closed and s.getvalue() == b'reopened'
# ---
# case: BytesIO closes under context management and validates buffers
from _io import BytesIO
with BytesIO() as s:
    s.writelines([b'a', memoryview(b'b')])
    assert s.getvalue() == b'ab'
assert s.closed
for call in (s.read, s.getvalue, s.getbuffer, s.flush, s.tell,
             s.readable, s.writable, s.seekable):
    try:
        call()
        assert False
    except ValueError:
        pass
try:
    BytesIO().write('text')
    assert False
except TypeError:
    pass
