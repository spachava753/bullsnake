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
        assert e.args == (() if args == (None,) else args)
for status, expected_args, expected_code in [((), (), None), ((1,), (1,), 1), ((1, 2), (1, 2), (1, 2))]:
    try:
        sys.exit(status)
        assert False
    except SystemExit as e:
        assert e.args == expected_args
        assert e.code == expected_code
existing = SystemExit(3)
try:
    sys.exit(existing)
except SystemExit as e:
    assert e is existing
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

# ---
# case: Unicode errors preserve codec arguments
for cls, source in [(UnicodeEncodeError, 'hé'), (UnicodeDecodeError, b'\xff')]:
    e = cls('utf-8', source, 0, 1, 'invalid')
    assert isinstance(e, UnicodeError)
    assert isinstance(e, ValueError)
    assert e.args == ('utf-8', source, 0, 1, 'invalid')
    assert e.encoding == 'utf-8'
    assert e.object is source
    assert e.start == 0
    assert e.end == 1
    assert e.reason == 'invalid'
try:
    UnicodeDecodeError('utf-8', 'text', 0, 1, 'invalid')
    assert False
except TypeError:
    pass

# ---
# case: codec exception subclasses and bare raises use constructor validation
class DecodeFailure(UnicodeDecodeError):
    pass
error = DecodeFailure('utf-8', b'\xff', 0, 1, 'invalid')
assert type(error) is DecodeFailure
assert error.object == b'\xff'
assert error.args == ('utf-8', b'\xff', 0, 1, 'invalid')
for cls in [UnicodeEncodeError, UnicodeDecodeError, DecodeFailure]:
    try:
        raise cls
        assert False
    except TypeError:
        pass
assert UnicodeEncodeError('utf-8', 'x', False, True, 'invalid').start == 0
assert type(UnicodeEncodeError('utf-8', 'x', False, True, 'invalid').start) is int

# ---
# case: exception text observes retained mutable arguments
payload = [1]
error = ValueError(payload)
payload.append(2)
assert error.args[0] is payload
assert str(error) == '[1, 2]'
error = ValueError(payload, 'detail')
payload.append(3)
assert str(error) == "([1, 2, 3], 'detail')"
assert str(SystemExit(ValueError('detail'))) == 'detail'
