try:
    print('abc', 'later', flush=True)
    assert False
except OSError:
    pass
