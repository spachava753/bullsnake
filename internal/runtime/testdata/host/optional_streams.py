import sys
assert sys.stdout.isatty()
assert not sys.stdout.seekable()
assert sys.stdout.flush() is None
with sys.stdout as stream:
    assert stream is sys.stdout
    assert stream.write('borrowed') == 8
assert sys.stdout.closed
