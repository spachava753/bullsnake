import builtins
from _io import _BufferedIOBase
builtins.lease_buffer = bytearray(4)
builtins.lease_view = memoryview(builtins.lease_buffer)
class Reader(_BufferedIOBase):
    def read(self, size):
        import host_failure
Reader().readinto(builtins.lease_view)
