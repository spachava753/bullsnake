# case: native hash and call descriptors reject incompatible receivers
for operation in [lambda: int.__hash__('text'), lambda: int.__hash__(), lambda: int.__hash__(1, 2), lambda: type(lambda: None).__call__(1), lambda: type.__call__(1)]:
    try:
        operation()
        assert False
    except TypeError:
        pass
class Default:
    pass
value = Default()
assert Default.__hash__(value) == hash(value)
assert value.__hash__() == hash(value)
class Override:
    def __hash__(self):
        raise AssertionError('object hash must bypass override')
    native_hash = object.__hash__
value = Override()
assert value.native_hash() == object.__hash__(value)
assert object.__dict__['__hash__'] is object.__hash__
assert int.__dict__['__hash__'] is int.__hash__
assert bool.__hash__(True) == 1
