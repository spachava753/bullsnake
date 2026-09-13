# case: exact prepared dictionaries override class-body closure reads
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        return {'value': 42}
def outer():
    value = 3
    class Sample(metaclass=Meta):
        result = value
    return Sample
assert outer().result == 42

# ---
# case: an absent prepared name falls back to its enclosing cell
class Meta(type):
    @classmethod
    def __prepare__(meta, name, bases):
        return {}
def outer():
    value = 3
    class Sample(metaclass=Meta):
        result = value
    return Sample
assert outer().result == 3
