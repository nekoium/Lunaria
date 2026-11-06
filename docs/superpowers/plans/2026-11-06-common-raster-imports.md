# Common raster imports implementation plan

**Goal:** Resolve #4 through #5 by adding WebP, AVIF, BMP, and TIFF import. Investigate HEIC/HEIF separately in #6.

**Design:** Register bundled decoders with Go's existing image registry. Keep `loadImage` as the shared entry point, content detection, premultiplied BGRA conversion, first-frame behavior, and the 40-megapixel check. Extend the picker and public format list. Extract the first animated WebP frame through the existing WebP decoder, retaining its canvas and blending. No new rendering path.

**Alternatives:** Windows codecs vary by installation. Bundled decoders provide predictable support with larger binaries and additional dependencies. SVG requires vector rendering and is excluded.

**Approved scope:** User requested these four formats as one commit, then a conditional HEIC/HEIF investigation.

- [x] Register `golang.org/x/image` BMP, TIFF, and WebP decoders alongside AVIF; choose versions compatible with Go 1.25.
- [x] Add small generated AVIF and WebP fixtures; generate BMP/TIFF test inputs with their encoders.
- [x] Test native bitmap creation, dimensions, BGRA pixels, alpha, misleading extensions, corrupt inputs, and oversized headers. Test animated WebP first-frame selection, canvas offsets, blending, and invalid frame bounds.
- [x] Validate the original Pexels AVIF locally; measure import time and binary size against baseline.
- [x] Update picker and docs; run tests, vet, and Windows build; review the final diff.
- [x] Commit once.

**Validation:** `go test ./...`, `go vet ./...`, and the Windows GUI build passed. The original #4 file is a 4000×6000 AVIF: approximately 1.7 seconds for the first import and 1.1 seconds for a subsequent import in a separate native test process. Go reported about 635 MiB allocated on the heap immediately after loading, including temporary decoder memory. These are local observations, not performance guarantees. The stripped executable grew from approximately 2.3 MiB to 7.4 MiB.

Rim candidates: `issue-4-image-import` and `common-raster-formats`. No GUI automation is needed to validate decoders; interactive picker behavior remains outside automated coverage.
