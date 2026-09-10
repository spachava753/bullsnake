# case: read seek and truncate count Python characters
from _io import StringIO
s = StringIO('a🙂\ud800bc')
assert s.tell() == 0
assert s.read(2) == 'a🙂'
assert s.tell() == 2
assert s.read(0) == ''
assert s.read(None) == '\ud800bc'
assert s.read() == ''
assert s.seek(1) == 1
assert s.read(-2) == '🙂\ud800bc'
assert s.seek(0, 1) == 5
assert s.seek(0, 2) == 5
assert s.truncate(3) == 3
assert s.tell() == 5
assert s.getvalue() == 'a🙂\ud800'
assert s.read() == ''
assert s.tell() == 5
assert s.truncate(20) == 20
assert s.getvalue() == 'a🙂\ud800'
assert s.write('') == 0
assert s.getvalue() == 'a🙂\ud800'
assert s.write('x') == 1
assert s.getvalue() == 'a🙂\ud800\0\0x'
assert s.seek(2) == 2
assert s.truncate() == 2
assert s.getvalue() == 'a🙂'
assert s.truncate(None) == 2
assert s.readable() and s.writable() and s.seekable()
# ---
# case: readline obeys configured newline and character limits
from _io import StringIO
for mode, lines in ((None, ['a\n','b\n','c\n','d']),
                    ('', ['a\r','b\r\n','c\n','d']),
                    ('\n', ['a\rb\r\n','c\n','d']),
                    ('\r', ['a\r','b\r','\r','c\r','d']),
                    ('\r\n', ['a\rb\r\r\n','c\r\n','d'])):
    s = StringIO('a\rb\r\nc\nd', newline=mode)
    for line in lines:
        assert s.readline(None) == line
    assert s.readline() == ''
s = StringIO('🙂\r\nx\u2028y\n', newline='')
assert s.readline(2) == '🙂\r'
assert s.readline(0) == ''
assert s.readline(1) == '\n'
assert s.readline(-1) == 'x\u2028y\n'
# ---
# case: sizes use index protocol and run before closed checks
from _io import StringIO
events = []
class Index:
    def __init__(self, value):
        self.value = value
    def __index__(self):
        events.append(self.value)
        return self.value
s = StringIO('abc')
assert s.read(Index(1)) == 'a'
assert s.seek(Index(0), Index(2)) == 3
assert s.truncate(Index(2)) == 2
assert events == [1, 0, 2, 2]
class Close:
    def __index__(self):
        s.close()
        return 1
try:
    s.read(Close())
    assert False
except ValueError:
    pass
# ---
# case: rejected positions preserve cursor and buffer
from _io import StringIO
s = StringIO('kept')
for call, kind in ((lambda: s.seek(-1), ValueError),
                   (lambda: s.seek(0, 3), ValueError),
                   (lambda: s.seek(1, 1), OSError),
                   (lambda: s.seek(-1, 2), OSError),
                   (lambda: s.seek(1 << 100), OverflowError),
                   (lambda: s.read(1 << 100), OverflowError),
                   (lambda: s.truncate(-1), ValueError),
                   (lambda: s.read(1.5), TypeError),
                   (lambda: s.read(size=1), TypeError),
                   (lambda: s.seek(None), TypeError)):
    try:
        call()
        assert False
    except kind:
        pass
assert s.tell() == 0
assert s.getvalue() == 'kept'
s.close()
for method in (s.read, s.readline, s.tell, s.truncate,
               s.readable, s.writable, s.seekable, lambda: s.seek(0)):
    try:
        method()
        assert False
    except ValueError:
        pass
