# case: text positions roundtrip multibyte characters and translated newlines
from _io import TextIOWrapper, BytesIO
s = TextIOWrapper(BytesIO(b'\xc3\xa9\r\n\xe9\x9b\xaa\rX'))
assert s.tell() == 0
assert s.read(1) == 'é'
start = s.tell()
assert start == 2
assert s.read(2) == '\n雪'
end = s.tell()
assert s.seek(start) == start
assert s.read(2) == '\n雪'
assert s.tell() == end
assert s.seek(0, 1) == end
assert s.read() == '\nX'
assert s.seek(0, 2) == 9
assert s.seek(0) == 0
assert s.newlines is None

# ---
# case: text positions preserve CRLF cuts and pending UTF8
from _io import TextIOWrapper, BytesIO
s = TextIOWrapper(BytesIO(b'a\r\nb'), newline='')
assert s.read(2) == 'a\r'
position = s.tell()
assert s.read() == '\nb'
s.seek(position)
assert s.read() == '\nb'
s = TextIOWrapper(BytesIO(b'abc'))
assert s.read(1) == 'a'
assert s.write('é') == 1
assert s.tell() == 5
assert s.seek(0) == 0
assert s.read() == 'abcé'

# ---
# case: text iteration disables telling until flush or exhaustion
from _io import TextIOWrapper, BytesIO, UnsupportedOperation
s = TextIOWrapper(BytesIO(b'a\nb'))
assert next(s) == 'a\n'
try:
    s.tell()
    assert False
except OSError:
    pass
s.flush()
assert s.tell() == 2
assert next(s) == 'b'
assert next(s, None) is None
assert s.tell() == 3
for whence in [1, 2]:
    try:
        s.seek(1, whence)
        assert False
    except UnsupportedOperation:
        pass
try:
    s.seek(-1)
    assert False
except ValueError:
    pass

# ---
# case: text truncation flushes encoded output and retains cursor
from _io import TextIOWrapper, BytesIO
raw = BytesIO()
s = TextIOWrapper(raw)
s.write('abcdef')
assert s.truncate(3) == 3
assert raw.getvalue() == b'abc' and s.tell() == 6
