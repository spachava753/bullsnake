# C3 multiple-inheritance behavior.
# case: cooperative diamond inheritance
class MROBase:
    side = 'base'
    def order(self):
        return 0

class MROLeft(MROBase):
    side = 'left'
    def order(self):
        return super().order() * 10 + 1

class MRORight(MROBase):
    side = 'right'
    def order(self):
        return super().order() * 10 + 2

class MRODiamond(MROLeft, MRORight):
    def order(self):
        return super().order() * 10 + 3

class MROReversed(MRORight, MROLeft):
    pass

bases = (MROLeft, MRORight)
class MROExpanded(*bases):
    pass

diamond = MRODiamond()
reversed_value = MROReversed()
expanded = MROExpanded()

assert diamond.order() == 213
assert reversed_value.order() == 12
assert expanded.order() == 21
assert diamond.side == 'left'
assert reversed_value.side == 'right'
assert super(MROLeft, diamond).order() == 2
assert MRODiamond.__bases__ == (MROLeft, MRORight)
assert MRODiamond.__base__ is MROLeft
assert MRODiamond.__mro__[0] is MRODiamond
assert MRODiamond.__mro__[1] is MROLeft
assert MRODiamond.__mro__[2] is MRORight
assert MRODiamond.__mro__[3] is MROBase
# ---
# case: later bases provide object protocols
class EmptyPrimary:
    pass

class InitializerMixin:
    def __init__(self, value):
        self.value = value

class AdditionMixin:
    def __add__(self, other):
        return self.value + other

class DescriptorForMRO:
    def __get__(self, instance, owner):
        if instance is None:
            return owner
        return instance.value * 2

class DescriptorMixin:
    doubled = DescriptorForMRO()

class ProtocolComposite(EmptyPrimary, InitializerMixin, AdditionMixin, DescriptorMixin):
    pass

composite = ProtocolComposite(7)
protocol_sum = composite + 5
protocol_descriptor = composite.doubled
class_descriptor = ProtocolComposite.doubled
pattern_matched = False
match composite:
    case AdditionMixin():
        pattern_matched = True

assert protocol_sum == 12
assert protocol_descriptor == 14
assert class_descriptor is ProtocolComposite
assert pattern_matched is True
# ---
# case: multiple user exception bases
class MROValueProblem(ValueError):
    pass

class MROTypeProblem(TypeError):
    pass

class MROCombinedProblem(MROValueProblem, MROTypeProblem):
    pass

caught_value_parent = False
caught_type_parent = False
try:
    raise MROCombinedProblem('value branch')
except ValueError:
    caught_value_parent = True
try:
    raise MROCombinedProblem('type branch')
except TypeError:
    caught_type_parent = True

assert caught_value_parent is True
assert caught_type_parent is True
# ---
# case: failed MRO construction follows class body execution
class ConflictX:
    pass

class ConflictY:
    pass

class ConflictXY(ConflictX, ConflictY):
    pass

class ConflictYX(ConflictY, ConflictX):
    pass

class_body_ran = False
try:
    class FailedConflict(ConflictXY, ConflictYX):
        global class_body_ran
        class_body_ran = True
except TypeError as error:
    conflict_message = f'{error!r}'
try:
    FailedConflict
except NameError:
    failed_name_absent = True

assert class_body_ran is True
assert failed_name_absent is True
assert conflict_message == 'TypeError("Cannot create a consistent method resolution order (MRO) for bases ConflictX, ConflictY")'
