# case: buffered random synchronizes alternating reads and writes
from _io import BufferedRandom, BytesIO
raw = BytesIO(b'abcdef')
s = BufferedRandom(raw, 4)
assert s.readable() and s.writable() and s.seekable()
assert s.read(2) == b'ab'
assert s.tell() == 2
assert s.write(b'XY') == 2
assert s.tell() == 4
assert s.read(2) == b'ef'
assert raw.getvalue() == b'abXYef'
assert s.seek(0) == 0
assert s.peek() == b'abXY'
assert s.read(1) == b'a'
s.flush()
assert raw.tell() == 1 and s.tell() == 1
assert s.read1(2) == b'bX'
assert s.write(b'!') == 1
assert s.truncate(5) == 5
assert raw.getvalue() == b'abX!e'
assert s.detach() is raw
assert raw.tell() == 4 and not raw.closed

# ---
# case: buffered random requires all raw capabilities
from _io import BufferedRandom, BytesIO, UnsupportedOperation
class Raw(BytesIO):
    def seekable(self):
        return False
try:
    BufferedRandom(Raw())
    assert False
except UnsupportedOperation:
    pass
class Raw(BytesIO):
    def writable(self):
        return False
try:
    BufferedRandom(Raw())
    assert False
except UnsupportedOperation:
    pass

# ---
# case: buffered random preserves readahead after failed rewind
from _io import BufferedRandom, BytesIO
class Raw(BytesIO):
    def seek(self, offset, whence=0):
        if self.fail:
            raise OSError('cannot seek')
        return super().seek(offset, whence)
raw = Raw(b'abcdef')
raw.fail = False
s = BufferedRandom(raw, 4)
assert s.read(1) == b'a'
raw.fail = True
try:
    s.flush()
    assert False
except OSError:
    pass
assert s.read(2) == b'bc'
raw.fail = False
s.flush()
assert raw.tell() == 3
s.close()
assert raw.closed
