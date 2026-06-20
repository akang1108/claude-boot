# Demo GIF

The animated demo GIF is generated with [vhs](https://github.com/charmbracelet/vhs).

## Setup

```bash
brew install vhs
```

## Regenerating the GIF

```bash
make build
vhs docs/demo.tape
```

Output is written to `docs/demo.gif`, which is referenced from `README.md`.

## Editing the demo

Edit `docs/demo.tape` to change what the recording does. Key tips:

- Add `Sleep` calls between keypresses to let the TUI render before the next frame is captured
- Start with generous delays (500ms+) and tighten once the flow looks right
- `Set Width` / `Set Height` control the terminal dimensions in the recording
