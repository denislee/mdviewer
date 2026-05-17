# mdviewer

A Wayland-native Markdown viewer written in Go, built on [Gio](https://gioui.org)
(direct Wayland backend — no XWayland) with [goldmark](https://github.com/yuin/goldmark)
for parsing.

## Build & Run

```sh
make run                  # builds and opens README.md
make run FILE=path.md     # opens a specific file
make build                # produce ./mdviewer
```

Or directly:

```sh
go build -o mdviewer .
./mdviewer path/to/file.md
```

## Vim keys

| key            | action                  |
| -------------- | ----------------------- |
| `j` / `↓`      | scroll down             |
| `k` / `↑`      | scroll up               |
| `Ctrl-d`       | half page down          |
| `Ctrl-u`       | half page up            |
| `Ctrl-f` / `Space` / `PgDn` | page down  |
| `Ctrl-b` / `PgUp` | page up              |
| `gg` / `Home`  | jump to top             |
| `G` / `End`    | jump to bottom          |
| `,`            | open / close settings   |
| `q`            | quit                    |

## Settings

Press `,` to open the settings overlay. It currently exposes a single
toggle: *Default viewer for .md files*. Enabling it writes
`~/.local/share/applications/mdviewer.desktop` pointing at the running
binary and runs `xdg-mime default mdviewer.desktop` for `text/markdown`
and `text/x-markdown`. Disabling removes those entries from
`mimeapps.list` and deletes the desktop file. Press `,` or `Esc` to
close the overlay.

## Rendering support

Headings, paragraphs, **bold**, *italic*, `inline code`, fenced code blocks,
ordered & unordered lists (nested), block quotes, horizontal rules, links,
autolinks. GitHub-flavored Markdown extensions enabled.

## Notes

Gio binds directly to `libwayland-client` on Linux when a Wayland compositor
is available, so the window is rendered through Wayland protocols, not via
GTK or XWayland.
