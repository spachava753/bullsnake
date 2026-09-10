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
