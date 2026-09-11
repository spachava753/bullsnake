# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Sequence, Mapping, Callable

sequence_alias = Sequence[int]
class Numbers(sequence_alias):
    def __len__(self):
        return 2
    def __getitem__(self, index):
        return (10, 20)[index]
assert Numbers.__bases__ == (Sequence,)
assert Numbers.__orig_bases__ == (sequence_alias,)
assert list(Numbers()) == [10, 20]
assert Numbers().count(20) == 1

class Dictionary(Mapping[str, int]):
    def __iter__(self):
        return iter(('value',))
    def __len__(self):
        return 1
    def __getitem__(self, key):
        if key == 'value':
            return 3
        raise KeyError(key)
assert Dictionary() == {'value': 3}

class Function(Callable[[int], str]):
    def __call__(self, value):
        return str(value)
assert Function()(12) == '12'
assert isinstance(Function(), Callable)
