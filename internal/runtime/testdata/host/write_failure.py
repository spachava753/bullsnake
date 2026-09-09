import sys
try:
    sys.stdout.write('hé🙂')
    assert False
except OSError as e:
    error_type = type(e).__name__
    error_args = e.args
    error_errno = e.errno
    error_filename = e.filename
    error_text = str(e)
