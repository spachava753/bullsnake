class Sample:
    pass
value = Sample()
try:
    value.__getstate__()
except RuntimeError as error:
    assert str(error) == 'import failed'
else:
    assert False
assert value.__getstate__() is None
import copyreg
assert copyreg._slotnames(Sample) is Sample.__slotnames__
