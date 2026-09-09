import sys
try:
    sys.stdout.write('a\ud800b')
    assert False
except UnicodeEncodeError as e:
    assert e.encoding == 'utf-8'
    assert e.object == 'a\ud800b'
    assert e.start == 1
    assert e.end == 2
    assert e.reason == 'surrogates not allowed'
try:
    sys.stdin.read()
    assert False
except UnicodeDecodeError as e:
    assert e.encoding == 'utf-8'
    assert e.object == b'\xff'
    assert e.start == 0
    assert e.end == 1
