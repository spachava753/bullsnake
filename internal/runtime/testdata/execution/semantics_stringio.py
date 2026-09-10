# case: native io imports share identity and real exception ancestry
import _io
import _io as repeated
assert _io is repeated
assert issubclass(_io.UnsupportedOperation, OSError)
assert issubclass(_io.UnsupportedOperation, ValueError)
assert _io.BlockingIOError is BlockingIOError
assert type(_io.StringIO).__name__ == 'type'
# ---
# case: StringIO starts at zero and overwrites characters rather than bytes
from _io import StringIO
s = StringIO('a🙂c')
assert type(s) is StringIO
assert s.write('é') == 1
assert s.getvalue() == 'é🙂c'
snapshot = s.getvalue()
assert s.write('中x') == 2
assert s.getvalue() == 'é中x'
assert snapshot == 'é🙂c'
assert s.write('!') == 1
assert s.getvalue() == 'é中x!'
assert s.write('') == 0
assert s.write('\ud800') == 1
assert s.getvalue() == 'é中x!\ud800'
# ---
# case: constructor binding and newline translation
from _io import StringIO
assert StringIO(None).getvalue() == ''
assert StringIO(initial_value='abc').getvalue() == 'abc'
for mode, expected in ((None, 'a\nb\nc\n'), ('', 'a\rb\r\nc\n'),
                       ('\n', 'a\rb\r\nc\n'), ('\r', 'a\rb\r\rc\r'),
                       ('\r\n', 'a\rb\r\r\nc\r\n')):
    s = StringIO(newline=mode)
    assert s.write('a\rb\r\nc\n') == 7
    assert s.getvalue() == expected
    if mode is None or mode == '':
        assert s.newlines == ('\r', '\n', '\r\n')
    else:
        assert s.newlines is None
assert StringIO('a\r\nb', newline=None).getvalue() == 'a\nb'
for mode in (None, ''):
    s = StringIO(newline=mode)
    assert s.newlines is None
    assert s.write('\r') == 1
    assert s.newlines == '\r'
    assert s.write('\n') == 1
    assert s.newlines == ('\r', '\n')
    assert s.getvalue() == ('\n\n' if mode is None else '\r\n')
s = StringIO('abc', newline='\r\n')
assert s.write('\n') == 1
assert s.write('!') == 1
assert s.getvalue() == '\r\n!'
# ---
# case: print can capture output without any configured host streams
from _io import StringIO
s = StringIO()
assert print('hé🙂', 42, sep='|', file=s, flush=True) is None
assert s.getvalue() == 'hé🙂|42\n'
assert not s.closed
assert s.isatty() is False
# ---
# case: StringIO owns its buffer and closes under context management
from _io import StringIO
s = StringIO('before')
with s as same:
    assert same is s
    value = s.getvalue()
assert s.closed
assert value == 'before'
assert s.close() is None
assert s.close() is None
for call in (s.getvalue, s.flush, s.isatty, lambda: s.write('x'),
             lambda: s.newlines, s.__enter__):
    try:
        call()
        assert False
    except ValueError:
        pass
try:
    with StringIO() as failed:
        raise LookupError('body')
except LookupError:
    assert failed.closed
# ---
# case: invalid StringIO inputs leave existing data intact
from _io import StringIO
for call in (lambda: StringIO(1), lambda: StringIO(newline=1),
             lambda: StringIO('a', initial_value='b'), lambda: StringIO(extra=1)):
    try:
        call()
        assert False
    except TypeError:
        pass
try:
    StringIO(newline='bad')
    assert False
except ValueError:
    pass
s = StringIO('kept')
for call in (lambda: s.write(1), lambda: s.write(text='x'), lambda: s.getvalue(1)):
    try:
        call()
        assert False
    except TypeError:
        pass
assert s.getvalue() == 'kept'
# ---
# case: StringIO reports closed access with structured ValueError arguments
from _io import StringIO
s = StringIO()
s.close()
try:
    s.getvalue()
    assert False
except ValueError as error:
    assert error.args == ('I/O operation on closed file',)
try:
    s.write(42)
    assert False
except TypeError as error:
    assert error.args == ("string argument expected, got 'int'",)
