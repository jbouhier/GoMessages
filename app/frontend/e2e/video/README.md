# Launch video

`./make.sh` builds `site/public/product/launch.mp4` (38.5 s, 1080p30) from the
real UI running on demo data.

1. `mock.mjs`: the demo backend in Node, serving the UI from `frontend/dist`
   (build it first). Same sample data as `demo.go`, with delays that scale for
   slow-motion recording.
2. `record.mjs`: plays each scene in headless Chrome (typing, ⌘K, photo viewer,
   reactions, shortcuts, themes) and saves timestamped frames. The page clock
   and CSS animations run at 1/`SLOW` speed so frames stay smooth.
3. `stage.html` + `render.mjs`: the composition (captions, window frame,
   camera moves, intro and end cards), rendered frame by frame to H.264.
4. `music.py`: the synthesized music bed.

Edit captions and timing in `SCENES`, camera moves in `CAM` (`stage.html`).
Re-render without re-recording: `node render.mjs`. Preview frames:
`node render.mjs --stills 5,12.5,20` writes `.work/still-*.jpg`.
