# Adapted from Lib/test/test_colorsys.py in CPython 3.14.7.
# See stdlib/README.md and LICENSES/CPython-3.14.txt.
#
# The unittest method and subTest wrapper were replaced by ordinary assertions.

import colorsys

values = [
    # rgb, yiq (invalid YIQ values clamped to RGB range)
    ((1.0, 0.0, 1.0), (0.0, 0.5, 1.0)),
    ((0.0, 1.0, 0.0), (0.25, -1.0, -1.0)),
    ((0.0, 0.0, 1.0), (0.0, -1.0, 0.5)),
]

for rgb, yiq in values:
    assert colorsys.yiq_to_rgb(*yiq) == rgb
