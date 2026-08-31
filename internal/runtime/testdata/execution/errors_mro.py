# C3 construction errors.
# case: method resolution order metadata is read only
# error: AttributeError
# message: "readonly attribute"
class ReadOnlyMRO:
    pass

ReadOnlyMRO.__mro__ = ()
# ---
# case: direct base metadata mutation is unsupported
# error: AttributeError
# message: "readonly attribute"
class ReadOnlyBase:
    pass

class ReadOnlyChild(ReadOnlyBase):
    pass

ReadOnlyChild.__bases__ = ()
# ---
# case: duplicate direct base
# error: TypeError
# message: "duplicate base class DuplicateMROBase"
class DuplicateMROBase:
    pass

class DuplicateMROClass(DuplicateMROBase, DuplicateMROBase):
    pass
# ---
# case: inconsistent method resolution order
# error: TypeError
# message: "Cannot create a consistent method resolution order (MRO) for bases MROX, MROY"
class MROX:
    pass

class MROY:
    pass

class MROXY(MROX, MROY):
    pass

class MROYX(MROY, MROX):
    pass

class InconsistentMRO(MROXY, MROYX):
    pass
# ---
# case: mixed builtin exception multiple inheritance boundary
# error: TypeError
# message: "multiple inheritance with built-in exception bases is not supported"
class ExceptionMixin:
    pass

class UnsupportedExceptionMix(ValueError, ExceptionMixin):
    pass
