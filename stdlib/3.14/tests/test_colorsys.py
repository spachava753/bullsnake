# Adapted from Lib/test/test_colorsys.py in CPython 3.14.7.
# See stdlib/README.md and LICENSES/CPython-3.14.txt.
#
# The selected unittest methods use ordinary assertions. The upstream
# assertTripleEqual helper is reproduced without its unittest dependency.

import colorsys


def assert_almost_equal(left, right):
    difference = left - right
    assert -0.0000001 < difference < 0.0000001


def assert_triple_equal(left, right):
    assert_almost_equal(left[0], right[0])
    assert_almost_equal(left[1], right[1])
    assert_almost_equal(left[2], right[2])


# test_yiq_to_rgb_clamping
values = [
    # rgb, yiq (invalid YIQ values clamped to RGB range)
    ((1.0, 0.0, 1.0), (0.0, 0.5, 1.0)),
    ((0.0, 1.0, 0.0), (0.25, -1.0, -1.0)),
    ((0.0, 0.0, 1.0), (0.0, -1.0, 0.5)),
]

for rgb, yiq in values:
    assert colorsys.yiq_to_rgb(*yiq) == rgb


# test_yiq_values
values = [
    # rgb, yiq
    ((0.0, 0.0, 0.0), (0.0, 0.0, 0.0)),  # black
    ((0.0, 0.0, 1.0), (0.11, -0.3217, 0.3121)),  # blue
    ((0.0, 1.0, 0.0), (0.59, -0.2773, -0.5251)),  # green
    ((0.0, 1.0, 1.0), (0.7, -0.599, -0.213)),  # cyan
    ((1.0, 0.0, 0.0), (0.3, 0.599, 0.213)),  # red
    ((1.0, 0.0, 1.0), (0.41, 0.2773, 0.5251)),  # purple
    ((1.0, 1.0, 0.0), (0.89, 0.3217, -0.3121)),  # yellow
    ((1.0, 1.0, 1.0), (1.0, 0.0, 0.0)),  # white
    ((0.5, 0.5, 0.5), (0.5, 0.0, 0.0)),  # grey
]

for rgb, yiq in values:
    assert_triple_equal(yiq, colorsys.rgb_to_yiq(*rgb))


# test_hls_values
values = [
    # rgb, hls
    ((0.0, 0.0, 0.0), (0, 0.0, 0.0)),  # black
    ((0.0, 0.0, 1.0), (4.0 / 6.0, 0.5, 1.0)),  # blue
    ((0.0, 1.0, 0.0), (2.0 / 6.0, 0.5, 1.0)),  # green
    ((0.0, 1.0, 1.0), (3.0 / 6.0, 0.5, 1.0)),  # cyan
    ((1.0, 0.0, 0.0), (0, 0.5, 1.0)),  # red
    ((1.0, 0.0, 1.0), (5.0 / 6.0, 0.5, 1.0)),  # purple
    ((1.0, 1.0, 0.0), (1.0 / 6.0, 0.5, 1.0)),  # yellow
    ((1.0, 1.0, 1.0), (0, 1.0, 0.0)),  # white
    ((0.5, 0.5, 0.5), (0, 0.5, 0.0)),  # grey
]

for rgb, hls in values:
    assert_triple_equal(hls, colorsys.rgb_to_hls(*rgb))
    assert_triple_equal(rgb, colorsys.hls_to_rgb(*hls))


# test_hls_nearwhite
values = (
    # rgb, hls: these do not work in reverse
    ((0.9999999999999999, 1, 1), (0.5, 1.0, 1.0)),
    ((1, 0.9999999999999999, 0.9999999999999999), (0.0, 1.0, 1.0)),
)

for rgb, hls in values:
    assert_triple_equal(hls, colorsys.rgb_to_hls(*rgb))
    assert_triple_equal((1.0, 1.0, 1.0), colorsys.hls_to_rgb(*hls))


# test_hsv_values
values = [
    # rgb, hsv
    ((0.0, 0.0, 0.0), (0, 0.0, 0.0)),  # black
    ((0.0, 0.0, 1.0), (4.0 / 6.0, 1.0, 1.0)),  # blue
    ((0.0, 1.0, 0.0), (2.0 / 6.0, 1.0, 1.0)),  # green
    ((0.0, 1.0, 1.0), (3.0 / 6.0, 1.0, 1.0)),  # cyan
    ((1.0, 0.0, 0.0), (0, 1.0, 1.0)),  # red
    ((1.0, 0.0, 1.0), (5.0 / 6.0, 1.0, 1.0)),  # purple
    ((1.0, 1.0, 0.0), (1.0 / 6.0, 1.0, 1.0)),  # yellow
    ((1.0, 1.0, 1.0), (0, 0.0, 1.0)),  # white
    ((0.5, 0.5, 0.5), (0, 0.0, 0.5)),  # grey
]

for rgb, hsv in values:
    assert_triple_equal(hsv, colorsys.rgb_to_hsv(*rgb))
    assert_triple_equal(rgb, colorsys.hsv_to_rgb(*hsv))

    # test 360 phase shift in hue
    h, s, v = hsv
    assert_triple_equal(rgb, colorsys.hsv_to_rgb(h + 1.0, s, v))
