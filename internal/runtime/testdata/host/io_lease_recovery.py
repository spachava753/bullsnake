import builtins
builtins.lease_view.release()
builtins.lease_buffer[:] = b'resized after Go error'
assert builtins.lease_buffer == b'resized after Go error'
