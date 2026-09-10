# case: text wrapper buffers encoded output and counts input characters
from _io import TextIOWrapper, BytesIO, _TextIOBase
raw = BytesIO()
s = TextIOWrapper(raw, encoding='utf-8', newline='\r\n')
assert isinstance(s, _TextIOBase)
assert s.buffer is raw and s.encoding == 'utf-8' and s.errors == 'strict'
assert s.readable() and s.writable() and s.seekable()
assert not s.line_buffering and not s.write_through
assert s.write('é\n雪') == 3
assert raw.getvalue() == b''
s.flush()
assert raw.getvalue() == b'\xc3\xa9\r\n\xe9\x9b\xaa'
assert s.newlines is None
assert s.detach() is raw and not raw.closed
try:
    s.write('x')
    assert False
except ValueError:
    pass

# ---
# case: text wrapper write through differs from line buffering
from _io import TextIOWrapper, BytesIO
class Buffer(BytesIO):
    def flush(self):
        self.flushes += 1
        return super().flush()
raw = Buffer()
raw.flushes = 0
s = TextIOWrapper(raw, write_through=True)
assert s.write('a') == 1
assert raw.getvalue() == b'a' and raw.flushes == 0
s = TextIOWrapper(raw, line_buffering=True)
s.write('b')
assert raw.getvalue() == b'a'
s.write('\r')
assert raw.getvalue() == b'ab\r' and raw.flushes == 1
s.close()
assert raw.closed and s.closed
assert s.close() is None

# ---
# case: text wrapper codecs enforce explicit encoding and error policy
from _io import TextIOWrapper, BytesIO
raw = BytesIO()
s = TextIOWrapper(raw, encoding='latin-1', write_through=True)
assert s.write('é') == 1 and raw.getvalue() == b'\xe9'
try:
    s.write('雪')
    assert False
except UnicodeEncodeError as error:
    assert error.encoding == 'latin-1' and error.object == '雪'
    assert error.start == 0 and error.end == 1
raw = BytesIO()
s = TextIOWrapper(raw, encoding='ascii', errors='replace', write_through=True)
assert s.write('éA') == 2 and raw.getvalue() == b'?A'
s = TextIOWrapper(raw, encoding='ascii', errors='ignore', write_through=True)
s.write('雪B')
assert raw.getvalue() == b'?AB'
try:
    TextIOWrapper(BytesIO(), encoding='locale')
    assert False
except PermissionError:
    pass
try:
    TextIOWrapper(BytesIO(), encoding='missing-codec')
    assert False
except LookupError:
    pass

# ---
# case: text wrapper closes buffer after flush failure
from _io import TextIOWrapper, BytesIO
class Buffer(BytesIO):
    def flush(self):
        raise OSError('flush failed')
raw = Buffer()
s = TextIOWrapper(raw)
s.write('x')
try:
    s.close()
    assert False
except OSError as error:
    assert str(error) == 'flush failed'
assert raw.closed and s.closed

# ---
# case: text wrapper constructor validates newline and truth flags
from _io import TextIOWrapper, BytesIO
class Truth:
    def __bool__(self):
        return True
s = TextIOWrapper(BytesIO(), line_buffering=Truth(), write_through=Truth())
assert s.line_buffering and s.write_through
for newline in ['bad', 1]:
    try:
        TextIOWrapper(BytesIO(), newline=newline)
        assert False
    except (ValueError, TypeError):
        pass
