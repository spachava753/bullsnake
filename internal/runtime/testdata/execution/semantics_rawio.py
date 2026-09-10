# case: raw read delegates to Python readinto with mutable storage
from _io import _RawIOBase, _IOBase
assert issubclass(_RawIOBase, _IOBase)
class Raw(_RawIOBase):
    def __init__(self):
        self.data = b'abcde'
        self.calls = []
    def readinto(self, buffer):
        self.calls.append(len(buffer))
        size = min(2, len(buffer), len(self.data))
        buffer[:size] = self.data[:size]
        self.data = self.data[size:]
        return size
r = Raw()
assert r.read(0) == b''
assert r.read(4) == b'ab'
assert r.readall() == b'cde'
assert r.read() == b''
assert r.calls[:2] == [0, 4]
try:
    r.read(None)
    assert False
except TypeError:
    pass
# ---
# case: raw read and readall preserve nonblocking results
from _io import _RawIOBase
class Raw(_RawIOBase):
    def readinto(self, buffer):
        return None
assert Raw().read(2) is None
assert Raw().readall() is None
class Partial(_RawIOBase):
    def __init__(self):
        self.calls = 0
    def read(self, size):
        self.calls += 1
        if self.calls == 1:
            raise InterruptedError(4, 'retry')
        return b'partial' if self.calls == 2 else None
p = Partial()
assert p.readall() == b'partial'
assert p.calls == 3
# ---
# case: raw read validates count and propagates index callback failures
from _io import _RawIOBase
class Count:
    def __index__(self):
        return 2
class Raw(_RawIOBase):
    def readinto(self, buffer):
        buffer[:2] = b'ok'
        return Count()
assert Raw().read(3) == b'ok'
class Invalid(_RawIOBase):
    def __init__(self, count):
        self.count = count
    def readinto(self, buffer):
        return self.count
for count in (-1, 4):
    try:
        Invalid(count).read(3)
        assert False
    except ValueError:
        pass
try:
    Invalid('bad').read(3)
    assert False
except TypeError:
    pass
# ---
# case: raw base abstract defaults match CPython rather than permissions
from _io import _RawIOBase
for call in (lambda: _RawIOBase().readinto(bytearray(1)),
             lambda: _RawIOBase().write(b'x')):
    try:
        call()
        assert False
    except NotImplementedError:
        pass
