# case: I/O bases have real inheritance and per-instance close state
from _io import _IOBase, _TextIOBase, UnsupportedOperation
for base in (_TextIOBase,):
    assert issubclass(base, _IOBase)
    assert isinstance(base(), _IOBase)
a = _IOBase()
b = _IOBase()
assert not a.closed and not b.closed
assert a.flush() is None
assert a.readable() is False
assert a.writable() is False
assert a.seekable() is False
assert a.isatty() is False
assert a.close() is None
assert a.close() is None
assert a.closed and not b.closed
try:
    a.flush()
    assert False
except ValueError:
    pass
for name in ('fileno', 'seek', 'truncate'):
    try:
        getattr(b, name)(0) if name != 'fileno' else b.fileno()
        assert False
    except UnsupportedOperation:
        pass
# ---
# case: inherited native methods bind through instances and super
from _io import _IOBase
events = []
class Stream(_IOBase):
    def __init__(self, value):
        self.value = value
        super().__init__()
    def flush(self):
        events.append(self.value)
        assert not self.closed
s = Stream('flushed')
assert s.__class__ is Stream
assert _IOBase.close(s) is None
assert events == ['flushed']
assert s.closed
assert s.close() is None
assert events == ['flushed']
class Failure(_IOBase):
    def flush(self):
        raise OSError('flush failed')
f = Failure()
try:
    f.close()
    assert False
except OSError as error:
    assert error.args == ('flush failed',)
assert f.closed
assert f.close() is None
# ---
# case: base context management dispatches Python close overrides
from _io import _IOBase
events = []
class Stream(_IOBase):
    def close(self):
        events.append('close')
        super().close()
with Stream() as s:
    assert not s.closed
assert s.closed and events == ['close']
try:
    with s:
        assert False
except ValueError:
    pass
# ---
# case: text base defaults and unsupported operations
from _io import _TextIOBase, UnsupportedOperation
s = _TextIOBase()
assert s.encoding is None and s.errors is None and s.newlines is None
for call in (s.read, s.readline, lambda: s.write('x'), s.detach):
    try:
        call()
        assert False
    except UnsupportedOperation:
        pass
try:
    s.closed = True
    assert False
except AttributeError:
    pass
# ---
# case: I/O class state is immutable but subclasses are mutable
from _io import _IOBase
try:
    _IOBase.marker = 1
    assert False
except TypeError:
    pass
class Child(_IOBase):
    pass
Child.marker = 1
assert Child().marker == 1
