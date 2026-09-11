# Project-owned behavior tests against unchanged CPython 3.14.7 io.py.
# These exercise the public Python ABC layer, not only the private _io module.
import io
import _io
import io as repeated
assert io is repeated
assert io.StringIO is _io.StringIO
assert io.BytesIO is _io.BytesIO
assert io.open is _io.open
assert io.UnsupportedOperation is _io.UnsupportedOperation
assert io.SEEK_SET == 0 and io.SEEK_CUR == 1 and io.SEEK_END == 2

for cls in [io.BytesIO, io.BufferedReader, io.BufferedWriter, io.BufferedRandom, io.BufferedRWPair]:
    assert issubclass(cls, io.BufferedIOBase)
    assert issubclass(cls, io.IOBase)
    assert not issubclass(cls, io.TextIOBase)
for cls in [io.StringIO, io.TextIOWrapper]:
    assert issubclass(cls, io.TextIOBase)
    assert issubclass(cls, io.IOBase)
    assert not issubclass(cls, io.RawIOBase)
assert issubclass(io.FileIO, io.RawIOBase)

with io.StringIO() as stream:
    assert isinstance(stream, io.TextIOBase)
    assert isinstance(stream, io.Reader)
    assert isinstance(stream, io.Writer)
    print('hé', 7, file=stream)
    stream.seek(0)
    assert stream.read() == 'hé 7\n'
assert stream.closed

binary = io.BytesIO()
with io.TextIOWrapper(binary, encoding='utf-8', newline='\r\n') as text:
    assert isinstance(text, io.Reader)
    assert isinstance(text, io.Writer)
    text.write('hé\n')
    text.flush()
    assert binary.getvalue() == b'h\xc3\xa9\r\n'
    text.seek(0)
    assert text.readline() == 'hé\r\n'
assert binary.closed

class Raw(io.RawIOBase):
    def __init__(self):
        super().__init__()
        self.source = io.BytesIO(b'abcd')
    def readable(self):
        return True
    def readinto(self, target):
        return self.source.readinto(target)
raw = Raw()
assert isinstance(raw, io.IOBase)
assert isinstance(raw, io.RawIOBase)
reader = io.BufferedReader(raw, 2)
assert reader.read(3) == b'abc'
assert reader.read() == b'd'
reader.close()
assert raw.closed

class Readable:
    def read(self, size=-1):
        return 'read'
class Writable:
    def write(self, value):
        return len(value)
class Disabled(Readable):
    read = None
assert isinstance(Readable(), io.Reader)
assert isinstance(Writable(), io.Writer)
assert not isinstance(Readable(), io.Writer)
assert not isinstance(Disabled(), io.Reader)

for protocol in [io.Reader, io.Writer]:
    try:
        protocol()
        assert False
    except TypeError:
        pass
    alias = protocol[str]
    assert alias.__origin__ is protocol
    assert alias.__args__ == (str,)

class ConcreteReader(io.Reader):
    def read(self, size=-1):
        return 'value'
assert ConcreteReader().read() == 'value'
