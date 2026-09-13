"""Project-owned behavior checks against unchanged CPython types.py."""
import types
from types import FunctionType, UnionType, new_class, prepare_class

assert FunctionType is type(lambda: None)
assert UnionType is type(int | str)
assert isinstance(int | str, UnionType)
assert isinstance(1, int | str)
assert types.GetSetDescriptorType is type(FunctionType.__code__)
assert types.MemberDescriptorType is type(FunctionType.__globals__)
assert types.CodeType is type((lambda: None).__code__)
assert types.new_class.__globals__ is types.__dict__

namespace = {}
def populate(target):
    target['value'] = 42
    namespace['target'] = target
Created = new_class('Created', (), {}, populate)
assert Created.value == 42
assert Created.__name__ == 'Created'
meta, prepared, keywords = prepare_class('Another')
assert meta is type
assert type(prepared) is dict
assert keywords == {}
assert types.resolve_bases((Created,)) == (Created,)
