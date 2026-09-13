"""Exercise member construction through unchanged enum class machinery."""
import enum
assert enum.STRICT is enum.FlagBoundary.STRICT
assert enum.STRICT.name == 'STRICT'
assert enum.STRICT.value == 'strict'
assert list(enum.FlagBoundary) == [enum.STRICT, enum.CONFORM, enum.EJECT, enum.KEEP]

class Color(enum.Enum):
    RED = 1
    BLUE = 2
    ALSO_RED = 1
assert Color.RED.name == 'RED' and Color.RED.value == 1
assert Color.ALSO_RED is Color.RED
assert list(Color) == [Color.RED, Color.BLUE]
assert str(Color.RED) == 'Color.RED'
assert repr(Color.RED) == '<Color.RED: 1>'

class Code(enum.IntEnum):
    READ = 1
    WRITE = 2
assert isinstance(Code.READ, int)
assert type(Code.READ) is Code
assert Code.READ == 1 and Code.READ + 2 == 3
assert str(Code.READ) == '1'
assert repr(Code.READ) == '<Code.READ: 1>'
assert Code.READ.name == 'READ' and Code.READ.value == 1

class Word(enum.StrEnum):
    FIRST = enum.auto()
    SECOND = 'custom'
assert isinstance(Word.FIRST, str)
assert type(Word.FIRST) is Word
assert Word.FIRST == 'first'
assert str(Word.FIRST) == 'first'
assert repr(Word.FIRST) == "<Word.FIRST: 'first'>"
assert Word.SECOND.name == 'SECOND' and Word.SECOND.value == 'custom'
assert list(Word) == [Word.FIRST, Word.SECOND]
