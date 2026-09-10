import _io
import _io as repeated
assert _io is repeated
assert not hasattr(_io, 'local')
_io.local = _io.StringIO()
assert _io.local.write('owned') == 5
assert _io.local.getvalue() == 'owned'
