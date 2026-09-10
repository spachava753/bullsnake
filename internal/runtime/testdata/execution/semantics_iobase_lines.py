# case: base readline uses binary read and optional peek
from _io import _IOBase
class Reader(_IOBase):
    def __init__(self):
        self.data = b'ab\ncd'
        self.calls = []
    def read(self, size):
        self.calls.append(size)
        result = self.data[:size]
        self.data = self.data[size:]
        return result
r = Reader()
assert r.readline() == b'ab\n'
assert r.calls == [1, 1, 1]
assert r.readline(1) == b'c'
assert r.readlines() == [b'd']
class Buffered(Reader):
    def peek(self, size):
        return self.data
r = Buffered()
assert r.readline() == b'ab\n'
assert r.calls == [3]
assert r.readline(None) == b'cd'
assert r.readline() == b''
# ---
# case: readlines invokes overridden iteration and counts each returned line
from _io import _IOBase
class Text(_IOBase):
    def __init__(self):
        self.lines = iter(['🙂\n', 'a\n', 'last'])
    def readline(self):
        return next(self.lines, '')
s = Text()
assert s.readlines(2) == ['🙂\n', 'a\n']
assert s.readlines(0) == ['last']
assert s.readlines() == []
# ---
# case: writelines calls each write override and retries interrupted writes
from _io import _IOBase
class Writer(_IOBase):
    def __init__(self):
        self.values = []
        self.interrupted = False
    def write(self, value):
        if not self.interrupted:
            self.interrupted = True
            raise InterruptedError(4, 'interrupted')
        self.values.append(value)
w = Writer()
assert w.writelines([b'a', b'b']) is None
assert w.values == [b'a', b'b']
w.writelines(range(2000))
assert len(w.values) == 2002
def failure():
    yield b'kept'
    raise LookupError('iterator failed')
try:
    w.writelines(failure())
    assert False
except LookupError:
    assert w.values[-1] == b'kept'
# ---
# case: readline validates binary results and propagates callback errors
from _io import _IOBase
class Invalid(_IOBase):
    def read(self, size):
        return 'text'
try:
    Invalid().readline()
    assert False
except OSError:
    pass
class InvalidPeek(_IOBase):
    def peek(self, size):
        return None
try:
    InvalidPeek().readline()
    assert False
except OSError:
    pass
