# case: structured exception arguments
payload = [1, 2]
e = ValueError(payload, 'detail')
assert e.args == (payload, 'detail')
assert e.args[0] is payload
assert ValueError().args == ()
assert ValueError('').args == ('',)
assert repr(ValueError()) == 'ValueError()'
assert repr(ValueError(42)) == 'ValueError(42)'
assert StopIteration(payload).args == (payload,)
assert StopIteration(payload).value is payload
class Custom(ValueError):
    pass
assert Custom(payload).args[0] is payload

# ---
# case: SystemExit keeps code and BaseException ancestry
assert issubclass(SystemExit, BaseException)
assert not issubclass(SystemExit, Exception)
assert SystemExit().code is None
assert SystemExit(None).args == (None,)
assert SystemExit('exit').code == 'exit'
assert SystemExit(1, 2).code == (1, 2)
import sys
for args in [(), (None,), (0,), (7,), ('failed',)]:
    try:
        sys.exit(*args)
        assert False
    except SystemExit as e:
        assert e.code == (args[0] if args else None)
        assert e.args == args
try:
    sys.exit(1, 2)
    assert False
except TypeError:
    pass
try:
    sys.exit(status=1)
    assert False
except TypeError:
    pass

# ---
# case: OSError structured fields and subclass selection
assert IOError is OSError
for number, cls in [(1, PermissionError), (2, FileNotFoundError), (13, PermissionError), (17, FileExistsError), (20, NotADirectoryError), (21, IsADirectoryError), (32, BrokenPipeError), (11, BlockingIOError), (4, InterruptedError), (110, TimeoutError)]:
    e = OSError(number, 'failure', 'visible.txt', None, 'target.txt')
    assert type(e) is cls
    assert isinstance(e, OSError)
    assert e.args == (number, 'failure')
    assert e.errno == number
    assert e.strerror == 'failure'
    assert e.filename == 'visible.txt'
    assert e.filename2 == 'target.txt'
assert OSError('failure').errno is None
assert OSError('failure').filename is None
assert OSError(2, 'missing', None).args == (2, 'missing', None)
assert type(PermissionError(2, 'missing')) is PermissionError
assert str(OSError(2, 'missing', 'a')) == "[Errno 2] missing: 'a'"
assert str(OSError(2, 'missing', 'a', None, 'b')) == "[Errno 2] missing: 'a' -> 'b'"
e = BlockingIOError(11, 'retry', 3)
assert e.characters_written == 3
assert e.filename is None
assert e.args == (11, 'retry', 3)
