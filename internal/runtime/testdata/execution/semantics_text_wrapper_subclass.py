# case: text wrapper iteration dispatches subclass readline through the VM
from _io import BytesIO, TextIOWrapper
class Lines(TextIOWrapper):
    def __init__(self):
        super().__init__(BytesIO(b'ignored'))
        self.lines = ['first\n', 'last', '']
    def readline(self, size=-1):
        return self.lines.pop(0)
stream = Lines()
assert iter(stream) is stream
assert next(stream) == 'first\n'
try:
    stream.tell()
    assert False
except OSError:
    pass
assert stream.readlines() == ['last']
assert stream.tell() == 0

# ---
# case: exact text wrapper iteration bypasses instance readline replacement
from _io import BytesIO, TextIOWrapper
stream = TextIOWrapper(BytesIO(b'real\n'))
stream.readline = lambda: 'replacement'
assert next(stream) == 'real\n'
assert next(stream, None) is None

# ---
# case: subclass iteration validates results and propagates callback failures
from _io import BytesIO, TextIOWrapper
class Lines(TextIOWrapper):
    def readline(self, size=-1):
        if self.fail:
            raise LookupError('line failed')
        return self.result
stream = Lines(BytesIO())
stream.fail = True
try:
    next(stream)
    assert False
except LookupError as error:
    assert str(error) == 'line failed'
stream.fail = False
for value in [b'bytes', None, 1]:
    stream.result = value
    try:
        next(stream)
        assert False
    except OSError:
        pass
stream.result = ''
assert next(stream, None) is None
assert stream.tell() == 0

# ---
# case: text wrapper subclass can iterate an overridden closed stream
from _io import BytesIO, TextIOWrapper
class Lines(TextIOWrapper):
    def readline(self, size=-1):
        return 'override'
stream = Lines(BytesIO())
stream.close()
assert next(stream) == 'override'

# ---
# case: subclass line overrides cannot bypass initialization or detach checks
from _io import BytesIO, TextIOWrapper
class Lines(TextIOWrapper):
    def __init__(self):
        pass
    def readline(self, size=-1):
        assert False
stream = Lines()
try:
    next(stream)
    assert False
except ValueError:
    pass
TextIOWrapper.__init__(stream, BytesIO())
stream.detach()
try:
    next(stream)
    assert False
except ValueError:
    pass
