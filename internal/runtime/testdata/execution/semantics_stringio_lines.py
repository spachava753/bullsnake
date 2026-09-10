# case: iteration uses and preserves the stream cursor
from _io import StringIO
s = StringIO('a\nb\r\nc', newline='')
assert iter(s) is s
assert s.__iter__() is s
assert next(s) == 'a\n'
assert s.__next__() == 'b\r\n'
assert list(s) == ['c']
assert next(s, None) is None
assert s.seek(0) == 0
assert [line for line in s] == ['a\n', 'b\r\n', 'c']
s.close()
for call in (lambda: iter(s), lambda: next(s), s.__iter__, s.__next__):
    try:
        call()
        assert False
    except ValueError:
        pass
# ---
# case: readlines hint stops only when character total exceeds hint
from _io import StringIO
s = StringIO('🙂\na\nb\n')
assert s.readlines(2) == ['🙂\n', 'a\n']
assert s.tell() == 4
assert s.readlines(None) == ['b\n']
assert s.readlines() == []
class Hint:
    def __index__(self):
        return 1
s.seek(0)
assert s.readlines(Hint()) == ['🙂\n']
s.seek(0)
assert s.readlines(0) == ['🙂\n', 'a\n', 'b\n']
# ---
# case: writelines consumes iterables incrementally without adding separators
from _io import StringIO
s = StringIO(newline='\r\n')
def lines():
    yield 'first\n'
    assert s.getvalue() == 'first\r\n'
    yield 'second'
assert s.writelines(lines()) is None
assert s.getvalue() == 'first\r\nsecond'
assert s.writelines('xy') is None
assert s.getvalue() == 'first\r\nsecondxy'
try:
    s.writelines(['kept', 42, 'never'])
    assert False
except TypeError:
    assert s.getvalue() == 'first\r\nsecondxykept'
def fail():
    yield '!'
    raise LookupError('iteration')
try:
    s.writelines(fail())
    assert False
except LookupError as error:
    assert str(error) == 'iteration'
assert s.getvalue() == 'first\r\nsecondxykept!'
# ---
# case: writelines checks closed state before asking for an iterator
from _io import StringIO
s = StringIO()
class Lines:
    def __iter__(self):
        raise AssertionError('must not run')
s.close()
try:
    s.writelines(Lines())
    assert False
except ValueError:
    pass
s = StringIO()
def close_midway():
    yield 'kept'
    s.close()
    yield 'denied'
try:
    s.writelines(close_midway())
    assert False
except ValueError:
    assert s.closed
