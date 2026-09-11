# case: native collection descriptors validate their receiver and arguments
for cls, value in [(list, [1]), (tuple, (1,)), (dict, {1: 2}), (str, 'a'), (bytes, b'a'), (bytearray, bytearray(b'a')), (range, range(1))]:
    length = cls.__dict__['__len__']
    iterator = cls.__dict__['__iter__']
    assert length is cls.__len__
    assert length(value) == 1
    assert len(list(iterator(value))) == 1
    for operation in [lambda: length(1), lambda: length(value, value), lambda: iterator(), lambda: length(value=value)]:
        try:
            operation()
            assert False
        except TypeError:
            pass
assert list.__contains__([1], 1)
assert not list.__contains__([1], 2)
assert getattr([1], '__len__')() == 1
assert getattr(iter([2]), '__next__')() == 2
