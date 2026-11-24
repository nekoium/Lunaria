# Lunaria — Performance Requirements

This document records the product decisions for issue [#8](https://github.com/nekoium/Lunaria/issues/8). The [build guide](superpowers/plans/2026-11-23-performance.md) describes the implementation and checks.

## Problem

Lunaria is a casual desktop widget. Typical users have 16 GB of RAM. Issue #8 reports more than 700 MB of memory at idle and visible lag when adding a 4000 × 6000 image. Both are unacceptable for this kind of app.

Previously, every image remained in memory at full source resolution. One 4000 × 6000 image needs about 96 MB (91.6 MiB) for one 32-bit pixel buffer. Decoding and native rendering add overhead. Immediate Go heap measurements do not establish actual idle process memory.

## Priorities

1. Stable memory use.
2. Smooth editing.
3. Fast image adding.

Sharpness has the lowest priority. Startup only needs to feel normal.

## Product decisions

### Memory

- With 20 images on the board, stable memory must stay under 100 MB. Under 50 MB is preferred.
- Measure stable memory after 30 seconds without input.
- Memory can rise while adding an image. Keep this rise as small as practical; there is no fixed ceiling.
- Memory returns to the stable level after adding finishes.
- Repeated adding and removing must not cause continuing memory growth.

### Editing

- Moving, resizing, and rotating feels instant, including while an image is loading.
- Large source files must not make editing existing images slower.

### Adding images

- Aim for an image to appear in under one second, including a 4000 × 6000 image.
- A loading indicator can appear first, but does not count as the image appearing.
- Keep supported formats working normally. A warning is an acceptable last resort for an uncommon format that cannot meet the timing goal.
- Preserve the order and positions of images added together.

### Image quality

- Images need no more detail than the screen can display. Zooming beyond the screen is not a requirement.
- Slight softness after enlargement is acceptable. Full sharpness is preferred but not required.

### File types

Support PNG, JPEG, GIF, WebP, AVIF, HEIC, HEIF, BMP, and TIFF. Support does not imply identical decoding cost. Keep the existing first-frame or first-page behavior and existing format limitations.

### Original images

- Keep an exact copy of each original file on disk.
- In edit mode, right-clicking an image offers **Download**.
- Download saves the original to the user's Downloads folder.
- Download never replaces an existing file.

## Scope

- One board with up to 20 images.
- The primary monitor at 1080p or similar.
- Windows, as in the current MVP.

4K/8K monitors, multiple monitors, zoom beyond the screen, and further startup optimization are outside this work. Reconsider them when users request them or report problems.

## Implementation decisions

These choices were made during implementation under the owner's instruction to complete the work without further questions.

- **Memory measurement:** use Task Manager's Details-tab **Memory (active private working set)** column. Record total working set and private committed memory separately; they are different measurements. The automated loader test uses total working set as a stricter proxy. It is not a substitute for measuring the built app.
- **Kept resolution:** keep a display copy fitting 55% of the primary screen, with the screen dimensions capped at 1920 × 1080 for this pixel budget. The largest display copy is 1056 × 594, about 2.5 MB of pixels. A small image is never enlarged. Enlarging the board image can make it softer; Download still returns the exact original.
- **Import behavior:** decode one image at a time in the background. Show **Adding image…** in the edit hint. Keep the board usable. Finishing an import must not reopen editing after the user has closed it, or interrupt an active drag.
- **Slow-format fallback:** the current bundled AVIF decoder does not meet the one-second goal for the large example. Keep AVIF support, process it in the background, and show a nonblocking slow-import notification once per session when an import exceeds one second. This is an explicit timing limitation, not a claim that a loading hint satisfies the goal. This work adds no codec installation requirement or new dependency.
- **Quitting:** wait for the active decoder to finish before graphics shutdown, discard queued work, and remove any original copy that was not added to the board. Already saved images remain intact.
- **Download names:** use `Lunaria-<stored name>.<real extension>`, then numbered suffixes on conflict. Determine the extension from file contents; an AVIF originally named `.jpg` downloads as `.avif`. Use the Windows known Downloads folder, including redirected folders. Show errors; successful downloads need no dialog.

## Rejected options

- **Retain full-resolution pixels:** the per-image cost cannot meet the memory budget.
- **Apply the stable memory ceiling during decoding:** most supported decoders need the full image temporarily. A strict ceiling would make some formats slow or unsupported.
- **Hide slow adding behind a placeholder:** a placeholder does not satisfy the timing goal.
- **Add a required native AVIF codec:** this work keeps the current self-contained distribution. AVIF remains a documented speed exception.

## Acceptance

- Functional tests preserve supported formats, alpha, small-image dimensions, saved geometry, and exact original bytes.
- Twenty-image memory measurements settle after 30 seconds and remain stable over ten reload cycles.
- A 20-image drawing pass at 1920 × 1080 averages under 16 ms.
- Existing-image dragging stays responsive during a large AVIF import.
- Download chooses the real extension, avoids replacement, and reports failures without leaving a partial output file.
- Report the remaining AVIF timing gap, temporary decoding peaks, and limits of automated UI checks honestly.
