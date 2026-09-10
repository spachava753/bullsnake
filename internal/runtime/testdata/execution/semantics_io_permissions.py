# case: filesystem entry points deny paths descriptors and custom openers
import _io
import builtins
assert builtins.open is _io.open
calls = []
def opener(path, flags):
    calls.append(path)
    return 3
for operation in [lambda: open('example.txt'), lambda: _io.open('example.txt', 'w', opener=opener), lambda: _io.open(0), lambda: _io.FileIO('example.txt'), lambda: _io.FileIO(1, closefd=False), lambda: _io.open_code('module.py')]:
    try:
        operation()
        assert False
    except PermissionError:
        pass
assert calls == []
class Path:
    def __fspath__(self):
        calls.append('path')
        return 'example.txt'
try:
    open(Path())
    assert False
except PermissionError:
    pass
assert calls == ['path']

# ---
# case: denied open still validates Python arguments
from _io import open, FileIO, open_code
for mode in ['', 'rr', 'rw', 'bt', 'z']:
    try:
        open('path', mode)
        assert False
    except ValueError:
        pass
for values in [{'mode': 'rb', 'encoding': 'utf-8'}, {'mode': 'rb', 'errors': 'ignore'}, {'mode': 'rb', 'newline': ''}, {'closefd': False}]:
    try:
        open('path', **values)
        assert False
    except ValueError:
        pass
for operation in [lambda: open(None), lambda: open('path', 1), lambda: open('path', buffering='x'), lambda: open_code(b'path'), lambda: FileIO('path', 'rt')]:
    try:
        operation()
        assert False
    except (TypeError, ValueError):
        pass
try:
    open(-1)
    assert False
except ValueError:
    pass

# ---
# case: text encoding is deterministic and preserves supplied objects
from _io import text_encoding
assert text_encoding(None) == 'utf-8'
assert text_encoding('ascii') == 'ascii'
value = object()
assert text_encoding(value) is value
class Index:
    def __index__(self):
        return 1
assert text_encoding(None, Index()) == 'utf-8'
try:
    text_encoding(None, 2 ** 40)
    assert False
except OverflowError:
    pass

# ---
# case: FileIO subclasses cannot acquire access by skipping initialization
from _io import FileIO, _RawIOBase
assert issubclass(FileIO, _RawIOBase)
class Empty(FileIO):
    def __init__(self):
        pass
s = Empty()
assert s.closed
assert s.close() is None
for operation in [s.read, s.readall, s.flush, s.fileno, s.tell, s.readable, s.writable, s.seekable]:
    try:
        operation()
        assert False
    except ValueError:
        pass
