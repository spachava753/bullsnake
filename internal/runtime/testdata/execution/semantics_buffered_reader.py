# case: buffered reader keeps readahead and logical positions
from _io import BytesIO, BufferedReader, _BufferedIOBase
raw = BytesIO(b'ab\ncdef')
s = BufferedReader(raw, 4)
assert isinstance(s, _BufferedIOBase)
assert s.raw is raw
assert s.readable() and s.seekable() and not s.writable()
assert s.peek() == b'ab\nc'
assert raw.tell() == 4 and s.tell() == 0
assert s.read(1) == b'a'
assert s.readline() == b'b\n'
assert s.read1(8) == b'c'
assert s.readinto(bytearray(0)) == 0
assert s.seek(-2, 1) == 2
assert s.read() == b'\ncdef'
assert s.read() == b''
assert s.seek(0) == 0
assert list(s) == [b'ab\n', b'cdef']
assert s.seek(1) == 1
assert s.detach() is raw
assert not raw.closed
for operation in [s.read, s.flush, s.close, s.tell]:
    try:
        operation()
        assert False
    except ValueError:
        pass

# ---
# case: buffered reader handles short reads and nonblocking progress
from _io import BufferedReader, _RawIOBase
class Raw(_RawIOBase):
    def __init__(self):
        self.chunks = [b'ab', None, b'c', b'']
        self.calls = 0
    def readable(self):
        return True
    def readinto(self, target):
        self.calls += 1
        chunk = self.chunks.pop(0)
        if chunk is None:
            return None
        target[:len(chunk)] = chunk
        return len(chunk)
raw = Raw()
s = BufferedReader(raw, 4)
assert s.read(5) == b'ab'
assert raw.calls == 2
assert s.read1() == b'c'
assert s.read(1) == b''
raw.chunks = [None, None, b'xy']
assert s.read(1) is None
assert s.read1() == b''
target = bytearray(4)
assert s.readinto1(target) == 2
assert target == b'xy\0\0'
raw.chunks = [None, None]
assert s.readinto(target) is None
assert s.readinto1(target) is None
assert s.close() is None and s.closed and raw.closed
assert s.close() is None
try:
    s.read(0)
    assert False
except ValueError:
    pass

# ---
# case: buffered reader readall delegates and preserves buffered prefix
from _io import BufferedReader, BytesIO
class Raw(BytesIO):
    def readall(self):
        self.called = True
        return self.read()
raw = Raw(b'abcdef')
s = BufferedReader(raw, 2)
assert s.peek(99) == b'ab'
assert s.read(1) == b'a'
assert s.read() == b'bcdef'
assert raw.called
class Index:
    def __index__(self):
        return 2
s = BufferedReader(BytesIO(b'xy'), buffer_size=Index())
assert s.read(Index()) == b'xy'
for size in [0, -1]:
    try:
        BufferedReader(BytesIO(), size)
        assert False
    except ValueError:
        pass

# ---
# case: buffered reader validates callback counts and releases reentrancy guard
from _io import BufferedReader, _RawIOBase
class Raw(_RawIOBase):
    def readable(self):
        return True
    def readinto(self, target):
        if self.bad:
            return len(target) + 1
        try:
            self.wrapper.read(1)
            assert False
        except RuntimeError:
            pass
        target[0] = 120
        return 1
raw = Raw()
raw.bad = True
s = BufferedReader(raw, 2)
raw.wrapper = s
try:
    s.read(1)
    assert False
except OSError:
    pass
raw.bad = False
assert s.read(1) == b'x'

# ---
# case: raw callback may release its temporary view
from _io import BufferedReader, _RawIOBase
class Raw(_RawIOBase):
    def readable(self):
        return True
    def readinto(self, target):
        target[0] = 97
        target.release()
        return 1
assert BufferedReader(Raw(), 2).read(1) == b'a'
