import sys
s = sys.stdout
assert s is sys.__stdout__
assert sys.stderr is sys.__stderr__
assert sys.stdin is sys.__stdin__
assert s.encoding == 'utf-8'
assert s.errors == 'strict'
assert not s.closed
assert isinstance(s, type(s))
assert s.writable()
assert not s.readable()
assert not s.seekable()
assert not s.isatty()
assert s.write('hé🙂\n') == 4
assert s.flush() is None
for operation in [lambda: s.seek(0), s.tell, s.truncate, s.fileno, s.read]:
    try:
        operation()
        assert False
    except OSError as e:
        assert isinstance(e, ValueError)
        assert type(e).__name__ == 'UnsupportedOperation'
        assert type(e).__module__ == 'io'
        assert repr(type(e)) == "<class 'io.UnsupportedOperation'>"
class Capture:
    def __init__(self):
        self.text = ''
    def write(self, text):
        self.text = text
        return len(text)
sys.stdout = Capture()
sys.stdout.write('redirected')
assert sys.stdout.text == 'redirected'
assert sys.__stdout__ is s
assert s.close() is None
assert s.close() is None
assert s.closed
for operation in [s.flush, s.readable, s.writable, s.seekable, s.isatty, s.tell]:
    try:
        operation()
        assert False
    except ValueError:
        pass
try:
    s.write('closed')
    assert False
except ValueError:
    pass
