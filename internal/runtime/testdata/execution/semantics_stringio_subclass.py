# case: StringIO participates in the text I/O hierarchy
from _io import StringIO, _IOBase, _TextIOBase, UnsupportedOperation
assert issubclass(StringIO, _TextIOBase)
assert issubclass(StringIO, _IOBase)
s = StringIO('abc')
assert isinstance(s, _TextIOBase)
assert StringIO.getvalue(s) == 'abc'
assert s.encoding is None and s.errors is None
assert s.line_buffering is False
for call in (s.fileno, s.detach):
    try:
        call()
        assert False
    except UnsupportedOperation:
        pass
# ---
# case: StringIO subclasses preserve native storage and Python overrides
from _io import StringIO
class Capture(StringIO):
    def __init__(self, prefix):
        super().__init__(prefix)
        self.seek(0, 2)
        self.calls = []
    def write(self, value):
        self.calls.append(value)
        return super().write(value.replace('one', 'ONE').replace('two', 'TWO').replace('x', 'X'))
s = Capture('prefix:')
assert type(s) is Capture
assert s.writelines(['one', 'two']) is None
assert s.calls == ['one', 'two']
assert s.getvalue() == 'prefix:ONETWO'
assert print('x', file=s) is None
assert s.getvalue() == 'prefix:ONETWOX\n'
StringIO.__init__(s, 'reset')
assert s.getvalue() == 'reset' and s.tell() == 0
assert s.calls == ['one', 'two', 'x', '\n']
s.close()
StringIO.__init__(s, 'reopened')
assert not s.closed and s.read() == 'reopened'
# ---
# case: subclass iteration invokes and validates overridden readline
from _io import StringIO
class Lines(StringIO):
    def readline(self, size=-1):
        return super().readline(size).replace('a', 'A').replace('b', 'B')
s = Lines('a\nb')
assert list(s) == ['A\n', 'B']
class Invalid(StringIO):
    def readline(self):
        return b'wrong'
try:
    next(Invalid())
    assert False
except OSError:
    pass
# ---
# case: uninitialized subclasses cannot use StringIO buffer methods
from _io import StringIO
class Uninitialized(StringIO):
    def __init__(self):
        pass
s = Uninitialized()
try:
    s.getvalue()
    assert False
except ValueError:
    pass
StringIO.__init__(s)
assert s.getvalue() == ''
