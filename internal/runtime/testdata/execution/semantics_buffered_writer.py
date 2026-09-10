# case: buffered writer delays small writes and flushes on seek detach close
from _io import BytesIO, BufferedWriter, BufferedReader, _BufferedIOBase
raw = BytesIO()
s = BufferedWriter(raw, 4)
assert isinstance(s, _BufferedIOBase)
assert not isinstance(s, BufferedReader)
assert s.writable() and not s.readable() and s.seekable()
assert s.write(b'ab') == 2
assert raw.getvalue() == b'' and s.tell() == 2
assert s.write(bytearray(b'cd')) == 2
assert raw.getvalue() == b'' and s.tell() == 4
assert s.write(b'efghi') == 5
assert raw.getvalue() == b'abcdefghi'
assert s.write(b'j') == 1
assert s.seek(0) == 0
assert raw.getvalue() == b'abcdefghij'
assert s.write(b'XY') == 2
assert s.truncate(5) == 5
assert raw.getvalue() == b'XYcde'
assert s.detach() is raw and not raw.closed
s = BufferedWriter(raw, 4)
s.writelines([b'1', b'2'])
s.close()
assert raw.closed and s.closed
assert s.close() is None

# ---
# case: buffered writer retries short writes and reports accepted nonblocking bytes
from _io import BufferedWriter, _RawIOBase
class Raw(_RawIOBase):
    def __init__(self):
        self.data = b''
        self.block = False
    def writable(self):
        return True
    def write(self, data):
        assert isinstance(data, memoryview) and data.readonly
        if self.block:
            return None
        count = min(2, len(data))
        self.data += bytes(data[:count])
        return count
raw = Raw()
s = BufferedWriter(raw, 4)
assert s.write(b'abcdefg') == 7
assert raw.data == b'abcd'
s.flush()
assert raw.data == b'abcdefg'
raw.block = True
assert s.write(b'xy') == 2
try:
    s.write(b'12345')
    assert False
except BlockingIOError as error:
    assert error.characters_written == 2
try:
    s.flush()
    assert False
except BlockingIOError as error:
    assert error.characters_written == 0
raw.block = False
s.flush()
assert raw.data == b'abcdefgxy12'
raw.block = True
try:
    s.write(b'ABCDE')
    assert False
except BlockingIOError as error:
    assert error.characters_written == 4
raw.block = False
s.flush()
assert raw.data == b'abcdefgxy12ABCD'

# ---
# case: buffered writer close attempts raw close after flush failure
from _io import BufferedWriter, _RawIOBase
class Raw(_RawIOBase):
    def writable(self):
        return True
    def write(self, data):
        raise OSError('failed write')
raw = Raw()
s = BufferedWriter(raw, 4)
s.write(b'x')
try:
    s.close()
    assert False
except OSError as error:
    assert str(error) == 'failed write'
assert raw.closed and s.closed
assert s.close() is None

# ---
# case: buffered writer snapshots mutable inputs and rejects invalid counts
from _io import BufferedWriter, _RawIOBase
class Raw(_RawIOBase):
    def writable(self):
        return True
    def write(self, data):
        if self.bad:
            return -1
        self.data += bytes(data)
        return len(data)
raw = Raw()
raw.bad = True
raw.data = b''
s = BufferedWriter(raw, 4)
value = bytearray(b'ab')
s.write(value)
value[0] = 122
try:
    s.flush()
    assert False
except OSError:
    pass
raw.bad = False
s.flush()
assert raw.data == b'ab'
