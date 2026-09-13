"""Project-owned checks against unchanged CPython copyreg.py."""
import copyreg

value = complex(3, 4)
constructor, arguments = copyreg.pickle_complex(value)
assert constructor is complex
assert constructor(*arguments) == value
assert copyreg.dispatch_table[complex] is copyreg.pickle_complex

calls = []
class Sample:
    def __init__(self):
        calls.append('init')

first = copyreg.__newobj__(Sample)
second = copyreg._reconstructor(Sample, object, None)
assert type(first) is type(second) is Sample
assert first is not second
assert calls == []
assert copyreg._slotnames(Sample) == []

def reduce_sample(value):
    return Sample, ()
copyreg.pickle(Sample, reduce_sample)
assert copyreg.dispatch_table[Sample] is reduce_sample
copyreg.add_extension('example', 'Sample', 42)
assert copyreg._extension_registry[('example', 'Sample')] == 42
assert copyreg._inverted_registry[42] == ('example', 'Sample')
copyreg.remove_extension('example', 'Sample', 42)
assert ('example', 'Sample') not in copyreg._extension_registry
