class Sample:
    pass
value = Sample()
assert value.__getstate__() is None
assert Sample.__slotnames__ == []
import copyreg
assert copyreg._slotnames(Sample) is Sample.__slotnames__
