# case: buffered pair reads and writes independent raw streams
from _io import BytesIO, BufferedRWPair, UnsupportedOperation
reader = BytesIO(b'ab\ncd')
writer = BytesIO()
s = BufferedRWPair(reader, writer, 4)
assert s.readable() and s.writable() and not s.seekable()
assert s.peek() == b'ab\nc'
assert s.readline() == b'ab\n'
assert s.read() == b'cd'
assert s.write(b'xy') == 2
assert writer.getvalue() == b''
s.flush()
assert writer.getvalue() == b'xy'
for operation in [s.tell, s.fileno, s.detach]:
    try:
        operation()
        assert False
    except UnsupportedOperation:
        pass
s.close()
assert reader.closed and writer.closed and s.closed
assert s.close() is None

# ---
# case: buffered pair closes both sides despite writer failure
from _io import BytesIO, BufferedRWPair
class Writer(BytesIO):
    def write(self, data):
        raise OSError('write failed')
reader = BytesIO(b'x')
writer = Writer()
s = BufferedRWPair(reader, writer, 4)
s.write(b'x')
try:
    s.close()
    assert False
except OSError:
    pass
assert reader.closed and writer.closed

# ---
# case: buffered pair terminal status consults writer then reader
from _io import BytesIO, BufferedRWPair
events = []
class Reader(BytesIO):
    def isatty(self):
        events.append('reader')
        return True
class Writer(BytesIO):
    def isatty(self):
        events.append('writer')
        return False
s = BufferedRWPair(Reader(), Writer())
assert s.isatty()
assert events == ['writer', 'reader']

# ---
# case: buffered pair validates size once and both capabilities before construction
from _io import BytesIO, BufferedRWPair
events = []
class Size:
    def __index__(self):
        events.append('size')
        return 4
class Reader(BytesIO):
    def readable(self):
        events.append('readable')
        return True
class Writer(BytesIO):
    def writable(self):
        events.append('writable')
        return True
s = BufferedRWPair(Reader(), Writer(), Size())
assert events == ['size', 'readable', 'writable', 'readable', 'writable']
