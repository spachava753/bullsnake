import builtins
from _io import BufferedReader, _RawIOBase
class Raw(_RawIOBase):
    def readable(self):
        return True
    def readinto(self, target):
        if self.fail:
            import host_failure
        target[0] = 120
        return 1
raw = Raw()
raw.fail = True
builtins.guarded_reader = BufferedReader(raw, 4)
builtins.guarded_reader.read(1)
