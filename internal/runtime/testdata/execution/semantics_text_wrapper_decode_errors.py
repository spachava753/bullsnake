# UTF-8 error boundaries follow CPython 3.14.7 Objects/stringlib/codecs.h
# and Objects/unicodeobject.c at 823f0323ee6ec1402088b73bce1a38473cac36dc.
# case: text decoder consumes valid prefixes as one malformed sequence
from _io import BytesIO, TextIOWrapper
for data, end, reason, replacement in [
    (b'\xc2X', 1, 'invalid continuation byte', '\ufffdX'),
    (b'\xe1\x80X', 2, 'invalid continuation byte', '\ufffdX'),
    (b'\xf1\x80\x80X', 3, 'invalid continuation byte', '\ufffdX'),
    (b'\xe1\x80', 2, 'unexpected end of data', '\ufffd'),
    (b'\xf1\x80\x80', 3, 'unexpected end of data', '\ufffd'),
    (b'\xe0\x80X', 1, 'invalid continuation byte', '\ufffd\ufffdX'),
    (b'\xed\xa0\x80', 1, 'invalid continuation byte', '\ufffd\ufffd\ufffd'),
    (b'\xf0\x80\x80X', 1, 'invalid continuation byte', '\ufffd\ufffd\ufffdX'),
    (b'\xf4\x90\x80X', 1, 'invalid continuation byte', '\ufffd\ufffd\ufffdX'),
    (b'\xffX', 1, 'invalid start byte', '\ufffdX'),
]:
    try:
        TextIOWrapper(BytesIO(data)).read()
        assert False
    except UnicodeDecodeError as error:
        assert error.object == data
        assert error.start == 0 and error.end == end
        assert error.reason == reason
    assert TextIOWrapper(BytesIO(data), errors='replace').read() == replacement
    assert TextIOWrapper(BytesIO(data), errors='ignore').read() == replacement.replace('\ufffd', '')

# ---
# case: split malformed prefixes preserve replacement positions and line endings
from _io import BytesIO, TextIOWrapper
class Chunks(BytesIO):
    def read1(self, size):
        return super().read1(1)
for errors, first in [('replace', '\ufffd'), ('ignore', '\n')]:
    stream = TextIOWrapper(Chunks(b'\xf1\x80\x80\r\nZ'), errors=errors)
    assert stream.read(1) == first
    position = stream.tell()
    tail = stream.read()
    assert tail == ('\nZ' if errors == 'replace' else 'Z')
    stream.seek(position)
    assert stream.read() == tail

# ---
# case: failed decoding discards the chunk without publishing partial text
from _io import BytesIO, TextIOWrapper
for encoding in ['utf-8', 'ascii']:
    for size in [-1, 2]:
        stream = TextIOWrapper(BytesIO(b'A\r\n\xff'), encoding=encoding)
        try:
            stream.read(size)
            assert False
        except UnicodeDecodeError:
            pass
        assert stream.newlines is None
        assert stream.read() == ''

# ---
# case: failed decoding retains incomplete bytes and pending CR from prior chunks
from _io import _BufferedIOBase, TextIOWrapper
class Chunks(_BufferedIOBase):
    def __init__(self, parts):
        self.parts = parts
    def readable(self):
        return True
    def read1(self, size):
        return self.parts.pop(0)
for parts, result, newlines in [
    ([b'\xe1', b'\x80X', b'\x80\x80', b''], '\u1000', None),
    ([b'\r', b'X\xff', b'\nY', b''], '\nY', '\r\n'),
]:
    stream = TextIOWrapper(Chunks(parts))
    try:
        stream.read(2)
        assert False
    except UnicodeDecodeError:
        pass
    assert stream.newlines is None
    assert stream.read(2) == result
    assert stream.newlines == newlines

# ---
# case: failed unbounded reads preserve previously decoded read ahead
from _io import BytesIO, TextIOWrapper
class Chunks(BytesIO):
    def read1(self, size):
        return super().read1(3)
stream = TextIOWrapper(Chunks(b'abc\xff'))
assert stream.read(1) == 'a'
try:
    stream.read()
    assert False
except UnicodeDecodeError:
    pass
assert stream.read() == 'bc'
