# case: print and Unicode input traverse the complete in memory wrapper stack
from _io import BytesIO, BufferedRandom, TextIOWrapper
raw = BytesIO()
binary = BufferedRandom(raw, 4)
text = TextIOWrapper(binary, encoding='utf-8', newline=None)
print('é', '雪', sep='\r\n', file=text, flush=True)
assert raw.getvalue() == b'\xc3\xa9\r\n\xe9\x9b\xaa\n'
assert text.tell() == 8
assert text.seek(0) == 0
assert text.read(1) == 'é'
position = text.tell()
assert text.readline() == '\n'
assert text.read() == '雪\n'
text.seek(position)
assert text.read() == '\n雪\n'
text.close()
assert text.closed and binary.closed and raw.closed

# ---
# case: raw buffer counts use the Python index protocol
from _io import BufferedReader, BufferedWriter, _RawIOBase
class Count:
    def __init__(self, value):
        self.value = value
    def __index__(self):
        return self.value
class Reader(_RawIOBase):
    def readable(self):
        return True
    def readinto(self, target):
        target[0] = 120
        return Count(1)
assert BufferedReader(Reader(), 4).read(1) == b'x'
class Writer(_RawIOBase):
    def writable(self):
        return True
    def write(self, data):
        self.data += bytes(data)
        return Count(len(data))
raw = Writer()
raw.data = b''
s = BufferedWriter(raw, 4)
assert s.write(b'abcdef') == 6
assert raw.data == b'abcdef'

# ---
# case: text and raw writes retry EINTR without duplicating completed output
from _io import BytesIO, BufferedWriter, TextIOWrapper
class Buffer(BytesIO):
    def write(self, data):
        if self.interrupted:
            self.interrupted = False
            raise InterruptedError(4, 'interrupted')
        return super().write(data)
raw = Buffer()
raw.interrupted = True
binary = BufferedWriter(raw, 4)
text = TextIOWrapper(binary)
print('abc', file=text, flush=True)
assert raw.getvalue() == b'abc\n'
raw = Buffer()
raw.interrupted = True
text = TextIOWrapper(raw, write_through=True)
assert text.write('é') == 1
assert raw.getvalue() == b'\xc3\xa9'
