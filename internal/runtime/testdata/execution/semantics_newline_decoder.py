# case: incremental newlines preserve split CRLF and Unicode
from _io import IncrementalNewlineDecoder
d = IncrementalNewlineDecoder(None, True)
assert d.newlines is None
assert d.decode('é\r') == 'é'
assert d.getstate() == (b'', 1)
assert d.decode('') == ''
assert d.decode('\n雪\rX\n') == '\n雪\nX\n'
assert d.newlines == ('\r', '\n', '\r\n')
assert d.decode('\r') == ''
assert d.decode('', final=True) == '\n'
assert d.getstate() == (b'', 0)
assert d.reset() is None
assert d.newlines is None
d.__init__(decoder=None, translate=False, errors=object())
assert d.decode('a\r') == 'a'
assert d.decode('\nb\r', False) == '\r\nb'
assert d.decode('', True) == '\r'
assert d.newlines == ('\r', '\r\n')

# ---
# case: newline decoder state delegates and packs pending CR
from _io import IncrementalNewlineDecoder
class Decoder:
    def __init__(self):
        self.calls = []
    def decode(self, value, final):
        self.calls.append((value, final))
        return 'x\r'
    def getstate(self):
        return ('buffer', 9)
    def setstate(self, value):
        self.state = value
        return 42
    def reset(self):
        return 43
wrapped = Decoder()
d = IncrementalNewlineDecoder(wrapped, True)
assert d.decode(b'x') == 'x'
assert wrapped.calls == [(b'x', False)]
assert d.getstate() == ('buffer', 19)
assert d.setstate(('other', 8)) == 42
assert wrapped.state == ('other', 4)
assert d.getstate() == ('buffer', 18)
assert d.reset() == 43
assert d.newlines is None
d = IncrementalNewlineDecoder(None, True)
d.setstate((None, 1))
assert d.decode('a') == '\na'
assert d.newlines == '\r'
d.setstate((b'', 0))
assert d.newlines == '\r'

# ---
# case: newline decoder validates state and decoded output
from _io import IncrementalNewlineDecoder
d = IncrementalNewlineDecoder(None, True)
for value in [b'x', None, 1]:
    try:
        d.decode(value)
        assert False
    except TypeError:
        pass
for value in [None, [], (), (b'',), (b'', 'x')]:
    try:
        d.setstate(value)
        assert False
    except TypeError:
        pass
class MissingInit(IncrementalNewlineDecoder):
    def __init__(self):
        pass
try:
    MissingInit().decode('')
    assert False
except ValueError:
    pass
class Truth:
    def __bool__(self):
        return True
d = IncrementalNewlineDecoder(None, Truth())
assert d.decode('\r', Truth()) == '\n'
class Bad:
    def decode(self, value, final):
        raise RuntimeError('decoder failed')
d = IncrementalNewlineDecoder(Bad(), True)
try:
    d.decode(b'')
    assert False
except RuntimeError as error:
    assert str(error) == 'decoder failed'
