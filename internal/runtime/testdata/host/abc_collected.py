from registry import Base, saved_references, saved_hashes, other_reference
from _abc import _get_dump
for reference, old_hash in zip(saved_references, saved_hashes):
    assert reference() is None
    assert hash(reference) == old_hash
    assert reference == reference
    assert reference.__callback__ is None
assert other_reference() is None
assert saved_references[0] != other_reference
registry, positive, negative, version = _get_dump(Base)
assert registry == set()
assert positive == set()
assert negative == set()
assert Base.__subclasses__() == []
