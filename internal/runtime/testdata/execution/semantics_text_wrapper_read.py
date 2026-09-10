# case: text wrapper decodes character sized reads and newline kinds
from _io import TextIOWrapper, BytesIO
s = TextIOWrapper(BytesIO(b'\xc3\xa9\r\n\xe9\x9b\xaa\rX\n'))
assert s.read(1) == 'é'
assert s.readline() == '\n'
assert s.read(1) == '雪'
assert s.read() == '\nX\n'
assert s.newlines == ('\r', '\n', '\r\n')
assert s.read() == ''
s = TextIOWrapper(BytesIO(b'a\r\nb\rc\n'), newline='')
assert list(s) == ['a\r\n', 'b\r', 'c\n']
s = TextIOWrapper(BytesIO(b'a\r\nb\nc\r\n'), newline='\r\n')
assert s.readline(2) == 'a\r'
assert s.readline() == '\nb\nc\r\n'
assert s.newlines is None

# ---
# case: text wrapper carries split UTF8 and CRLF across binary reads
from _io import TextIOWrapper, _BufferedIOBase
class Buffer(_BufferedIOBase):
    def __init__(self):
        self.parts = [b'\xc3', b'\xa9\r', b'\n', b'\xe9\x9b', b'\xaa', b'']
    def readable(self):
        return True
    def read1(self, size):
        return self.parts.pop(0)
s = TextIOWrapper(Buffer())
assert s.read(2) == 'é\n'
assert s.readline() == '雪'
assert s.newlines == '\r\n'

# ---
# case: text wrapper read errors retain structured codec arguments
from _io import TextIOWrapper, BytesIO
try:
    TextIOWrapper(BytesIO(b'A\xff')).read()
    assert False
except UnicodeDecodeError as error:
    assert error.encoding == 'utf-8'
    assert error.object == b'A\xff'
    assert error.start == 1 and error.end == 2
assert TextIOWrapper(BytesIO(b'A\xffB'), errors='replace').read() == 'A�B'
assert TextIOWrapper(BytesIO(b'A\xffB'), errors='ignore').read() == 'AB'
assert TextIOWrapper(BytesIO(b'\xe9'), encoding='latin-1').read() == 'é'
try:
    TextIOWrapper(BytesIO(b'\xc3')).read()
    assert False
except UnicodeDecodeError as error:
    assert error.reason == 'unexpected end of data'

# ---
# case: text wrapper distinguishes all and bounded nonblocking reads
from _io import TextIOWrapper, _BufferedIOBase
class Buffer(_BufferedIOBase):
    def readable(self):
        return True
    def read(self, size=-1):
        return None
    def read1(self, size):
        return None
s = TextIOWrapper(Buffer())
try:
    s.read()
    assert False
except BlockingIOError:
    pass
try:
    s.read(1)
    assert False
except TypeError:
    pass

# ---
# case: text wrapper flushes pending output before reading
from _io import TextIOWrapper, BytesIO
raw = BytesIO()
s = TextIOWrapper(raw)
s.write('é')
assert raw.getvalue() == b''
assert s.read(0) == ''
assert raw.getvalue() == b'\xc3\xa9'
