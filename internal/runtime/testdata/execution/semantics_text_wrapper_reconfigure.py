# case: text reconfiguration flushes old bytes before changing encoding
from _io import TextIOWrapper, BytesIO
raw = BytesIO()
s = TextIOWrapper(raw, encoding='utf-8', errors='replace')
s.write('é')
assert s.reconfigure(encoding='latin-1', write_through=True) is None
assert raw.getvalue() == b'\xc3\xa9'
assert s.encoding == 'latin-1' and s.errors == 'strict'
assert s.write('é') == 1
assert raw.getvalue() == b'\xc3\xa9\xe9'
s.reconfigure(newline='\r\n', line_buffering=True)
s.write('\n')
assert raw.getvalue() == b'\xc3\xa9\xe9\r\n'
s.reconfigure(newline=None)
s.write('\n')
assert raw.getvalue() == b'\xc3\xa9\xe9\r\n\n'

# ---
# case: text reconfiguration protects existing decoded input
from _io import TextIOWrapper, BytesIO, UnsupportedOperation
s = TextIOWrapper(BytesIO(b'abc'))
assert s.read(1) == 'a'
for values in [{'encoding': 'ascii'}, {'errors': 'ignore'}, {'newline': None}]:
    try:
        s.reconfigure(**values)
        assert False
    except UnsupportedOperation:
        pass
s.reconfigure(line_buffering=True, write_through=True)
assert s.read() == 'bc'
s.seek(0)
s.reconfigure(encoding='ascii')
assert s.read() == 'abc'

# ---
# case: text reconfiguration does not apply options after a failed flush
from _io import TextIOWrapper, BytesIO
class Buffer(BytesIO):
    def flush(self):
        if self.fail:
            raise OSError('flush failed')
raw = Buffer()
raw.fail = True
s = TextIOWrapper(raw)
try:
    s.reconfigure(encoding='ascii', write_through=True)
    assert False
except OSError:
    pass
assert s.encoding == 'utf-8' and not s.write_through
raw.fail = False
class Index:
    def __index__(self):
        return 1
s.reconfigure(write_through=Index())
assert s.write_through

# ---
# case: text stream status delegates to mutable binary methods
from _io import TextIOWrapper, BytesIO
class Buffer(BytesIO):
    def writable(self):
        return self.allowed
raw = Buffer()
raw.allowed = True
s = TextIOWrapper(raw)
raw.allowed = False
assert not s.writable()

# ---
# case: text reconfiguration is allowed after complete reads and line exhaustion
from _io import TextIOWrapper, BytesIO
for operation in ['all', 'bounded then all', 'lines', 'unterminated line']:
    stream = TextIOWrapper(BytesIO(b'a\r\nb'))
    if operation == 'all':
        assert stream.read() == 'a\nb'
    elif operation == 'bounded then all':
        assert stream.read(1) == 'a'
        assert stream.read() == '\nb'
    elif operation == 'lines':
        assert list(stream) == ['a\n', 'b']
    else:
        assert stream.readline() == 'a\n'
        assert stream.readline() == 'b'
    assert stream.newlines == '\r\n'
    stream.reconfigure(encoding='ascii', newline='\r')
    assert stream.encoding == 'ascii'
    assert stream.newlines is None
    assert stream.write('\n') == 1
    stream.flush()
    assert stream.buffer.getvalue() == b'a\r\nb\r'

# ---
# case: exhausted bounded reads still protect decoder configuration
from _io import TextIOWrapper, BytesIO, UnsupportedOperation
for size in [3, 4]:
    stream = TextIOWrapper(BytesIO(b'abc'))
    assert stream.read(size) == 'abc'
    try:
        stream.reconfigure(encoding='ascii')
        assert False
    except UnsupportedOperation:
        pass
    assert stream.read() == ''
    stream.reconfigure(encoding='ascii')

# ---
# case: a failed first decode does not prevent reconfiguration
from _io import TextIOWrapper, BytesIO
for size in [-1, 1]:
    stream = TextIOWrapper(BytesIO(b'\xff'))
    try:
        stream.read(size)
        assert False
    except UnicodeDecodeError:
        pass
    stream.reconfigure(encoding='latin-1')
    assert stream.encoding == 'latin-1'
