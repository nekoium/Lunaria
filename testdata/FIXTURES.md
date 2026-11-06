# Image fixtures

`sample.png` is the original native smoke-test fixture.

The import tests add synthetic images, with no external photographs:

- `sample.avif`: 4×2 lossless image encoded with `github.com/gen2brain/avif` v0.6.0. Every pixel is RGB (240, 30, 10); the top row has alpha 255 and the bottom row has alpha 128.
- `sample.webp`: the same 4×2 image encoded losslessly with FFmpeg's `libwebp` encoder.
- `animated.webp`: two 4×2 frames, red followed by green, encoded losslessly with FFmpeg's `libwebp_anim` encoder. Tests verify that import selects red.

PNG, JPEG, GIF, BMP, and TIFF inputs are generated in the tests. The GIF includes a second green frame. Animated WebP canvas tests wrap the lossless still fixture with synthetic frame offsets and blending flags.
