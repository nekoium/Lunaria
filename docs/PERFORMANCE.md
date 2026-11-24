# Issue 8 implementation and measurements

## Behavior

Lunaria keeps bounded display bitmaps while preserving exact originals in board storage. The cache fits 55% of the primary screen, with screen dimensions capped at 1920 × 1080 for the pixel budget. Small images stay at their source resolution. Display enlargement can soften detail; Download copies the original bytes.

`readImage` reduces decoded pixels before constructing the retained BGRA buffer and GDI+ bitmap. `loadImage` releases unused decoder memory to Windows after decoding. This reduces retained memory, but does not eliminate the full-resolution decoding peak.

Normal file and clipboard imports run one at a time off the UI thread. The UI owns the request queue and board state, receives results through a channel, and applies results on its existing 150 ms timer. Clipboard data is copied before ownership is released. PNG encoding of clipboard bitmaps also runs in the worker. Completion waits until a drag or modal dialog ends, and preserves the current edit-mode state. Shutdown waits for the active decoder before GDI+ cleanup and removes its uncommitted stored original.

The importing hint and once-per-session slow-import notification provide feedback. They do not satisfy the one-second timing goal. The bundled AVIF decoder remains a speed exception; there is no new codec dependency or installation requirement.

Download uses the Windows known Downloads folder, detects the real format, and creates output exclusively with numbered conflicts. Failed copies are closed and removed. Existing output is never replaced.

## Measurements

These are local observations, not performance guarantees. Memory units below are MiB (1,048,576 bytes). The product goal is under 100 MB of active private working set after 30 seconds idle, preferably under 50 MB.

| Check | Result | Meaning |
| --- | --- | --- |
| Before: 3 large AVIF images in the loader test | 1320.28 MiB total working set after idle | Reproduces the memory problem in a test process; not app memory |
| Before: drawing those 3 images | 769.63 ms per drawing pass | Full-resolution sources make shrinking expensive |
| After: 20 images in the loader test, 7 AVIF + 7 JPEG + 6 PNG | 88.71 MiB total working set after idle | Retained pixels and decoder temporaries are reduced |
| After: 20-image drawing pass at 1920 × 1080 | 3.56 ms average | Measures image drawing, not the full desktop rendering pipeline |
| After: ten dispose/reload cycles | First round 97.11 MiB; last round 97.08 MiB | No continuing working-set growth in this run; the drawing surface remains allocated during these rounds |
| Large JPEG and PNG loads in that run | Under one second each | Low-level loading only, excluding storage copy, UI timer delay, and final presentation |
| Large AVIF loads in that run | About 2.8–5.2 seconds | The one-second target fails; the optional loader measurement intentionally stays red |
| Background AVIF responsiveness test | Queue 0.00 ms at displayed precision; completion 2.01 seconds | Queuing returns immediately; this separate run is not directly comparable to the loader timings |
| Dragging during that background import | 165 frames; 1.41 ms average; 5.83 ms maximum | Exercises repeated hidden-window native rendering while AVIF decodes |
| Built app: 20-image board after 30 seconds idle | 44.98 MiB active private working set | Corresponds to Task Manager's Details-tab memory metric; below 50 MB in this run |
| Same built app | 77.77 MiB total working set; 91.77 MiB private committed | These are different memory measures and must not be treated as interchangeable |
| Same built app: peak working set | 770.54 MiB | Large temporary decoding spikes remain |
| Same built app: native message response | 7.09 ms | The idle native message loop responds |
| Native smoke check | Passed | Existing native geometry, edit operations, Shell-drop payload, and surface-recreation checks pass |

The built app ran on a 2560 × 1600 primary display with the 1080p image-cache cap. All originals, transforms, and test storage were isolated from the user's board. Its 20 images were positioned off screen to avoid covering the user's desktop. Native windows and image resources were retained, but this check does not establish memory or drawing behavior for every on-screen arrangement. It was not a subjective visual review.

The loader test and app have different process overhead, DPI awareness, rendering surfaces, and memory counters. Baseline and post-change image counts also differ. Do not derive a precise percentage improvement from those rows.

The AVIF fixture is a current AVIF rendition of the Pexels photograph linked in issue #4, verified as 4000 × 6000. The GitHub attachment in that issue is only an error screenshot. Byte equality with the original historical download is not established.

## Reproduction and remaining limits

[CONTRIBUTING.md](../CONTRIBUTING.md) and the [implementation guide](superpowers/plans/2026-11-23-performance.md) contain commands and fixture setup. Local logs are under ignored `docs/verification/issue-8/`; downloaded media stays under ignored `.tmp/issue-8/`.

The standard suite and `go vet` pass. The Windows executable builds and native smoke passes. Tests cover small-image sizing, alpha, original bytes, queued clipboard ownership, ordering, placement, active drags, closed editing, failure cleanup, shutdown, output conflicts, invalid names, and known-folder lookup. The Go race detector was not run; `gcc` was not available on the tool environment's PATH.

The one-second AVIF goal remains unmet. Temporary decoding peaks remain high. Startup restoration still decodes saved originals sequentially. Automated native checks do not establish subjective smoothness, visual fidelity on every Explorer layout, end-to-end import timing for all formats, or support for 4K/8K and multiple monitors.
