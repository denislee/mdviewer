package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/richtext"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmext "github.com/yuin/goldmark/extension"
	gmtext "github.com/yuin/goldmark/text"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var (
	colBg       = color.NRGBA{R: 0xfa, G: 0xfa, B: 0xfa, A: 0xff}
	colFg       = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	colMuted    = color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xff}
	colLink     = color.NRGBA{R: 0x10, G: 0x66, B: 0xcc, A: 0xff}
	colCodeBg   = color.NRGBA{R: 0xed, G: 0xed, B: 0xed, A: 0xff}
	colCodeFg   = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff}
	colQuoteBar = color.NRGBA{R: 0xb0, G: 0xb0, B: 0xb0, A: 0xff}
	colRule     = color.NRGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
	colPanel    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colScrim    = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x80}
	colErr      = color.NRGBA{R: 0xc0, G: 0x20, B: 0x20, A: 0xff}
	colOk       = color.NRGBA{R: 0x20, G: 0x80, B: 0x40, A: 0xff}
)

func main() {
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		log.Fatal("usage: mdviewer <file.md>")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	blocks := parseMarkdown(src)

	go func() {
		if err := run(path, blocks); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(title string, blocks []block) error {
	w := new(app.Window)
	w.Option(
		app.Title("mdviewer — "+title),
		app.Size(unit.Dp(900), unit.Dp(720)),
	)

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops
	list := &widget.List{List: layout.List{Axis: layout.Vertical}}
	vs := &vimState{}
	st := &settingsState{}
	st.toggle.Value = isDefaultViewer()

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			// Full-window focusable area for key events.
			area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
			event.Op(gtx.Ops, vs)
			gtx.Execute(key.FocusCmd{Tag: vs})
			handleKeys(gtx, list, vs, st, len(blocks), w)
			area.Pop()

			paint.Fill(gtx.Ops, colBg)
			layout.Inset{
				Top: unit.Dp(8), Bottom: unit.Dp(8),
				Left: unit.Dp(28), Right: unit.Dp(28),
			}.Layout(gtx, func(gtx C) D {
				return material.List(th, list).Layout(gtx, len(blocks), func(gtx C, i int) D {
					return blocks[i].layout(gtx, th)
				})
			})

			if st.open {
				layoutSettings(gtx, th, st)
			}

			e.Frame(gtx.Ops)
		}
	}
}

// -----------------------------------------------------------------------------
// Vim key handling
// -----------------------------------------------------------------------------

type vimState struct {
	lastG bool
}

func handleKeys(gtx C, list *widget.List, vs *vimState, st *settingsState, n int, w *app.Window) {
	filters := []event.Filter{
		key.FocusFilter{Target: vs},
		key.Filter{Name: "J", Optional: key.ModShift | key.ModCtrl},
		key.Filter{Name: "K", Optional: key.ModShift | key.ModCtrl},
		key.Filter{Name: "G", Optional: key.ModShift | key.ModCtrl},
		key.Filter{Name: "D", Optional: key.ModCtrl},
		key.Filter{Name: "U", Optional: key.ModCtrl},
		key.Filter{Name: "F", Optional: key.ModCtrl},
		key.Filter{Name: "B", Optional: key.ModCtrl},
		key.Filter{Name: "Q"},
		key.Filter{Name: ","},
		key.Filter{Name: key.NameSpace, Optional: key.ModShift},
		key.Filter{Name: key.NameHome},
		key.Filter{Name: key.NameEnd},
		key.Filter{Name: key.NamePageDown},
		key.Filter{Name: key.NamePageUp},
		key.Filter{Name: key.NameDownArrow},
		key.Filter{Name: key.NameUpArrow},
		key.Filter{Name: key.NameEscape},
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}
		if ke.State != key.Press {
			continue
		}
		shift := ke.Modifiers.Contain(key.ModShift)
		ctrl := ke.Modifiers.Contain(key.ModCtrl)
		if ke.Name == "," {
			st.open = !st.open
			if st.open {
				st.toggle.Value = isDefaultViewer()
				st.msg = ""
			}
			vs.lastG = false
			w.Invalidate()
			continue
		}
		if st.open {
			if ke.Name == key.NameEscape || ke.Name == "Q" {
				st.open = false
				w.Invalidate()
			}
			continue
		}
		switch ke.Name {
		case "J", key.NameDownArrow:
			list.ScrollBy(0.25)
			vs.lastG = false
		case "K", key.NameUpArrow:
			list.ScrollBy(-0.25)
			vs.lastG = false
		case "D":
			if ctrl {
				list.ScrollBy(3)
			}
			vs.lastG = false
		case "U":
			if ctrl {
				list.ScrollBy(-3)
			}
			vs.lastG = false
		case "F":
			if ctrl {
				list.ScrollBy(6)
			}
			vs.lastG = false
		case "B":
			if ctrl {
				list.ScrollBy(-6)
			}
			vs.lastG = false
		case key.NameSpace, key.NamePageDown:
			list.ScrollBy(6)
			vs.lastG = false
		case key.NamePageUp:
			list.ScrollBy(-6)
			vs.lastG = false
		case "G":
			switch {
			case shift:
				list.ScrollTo(n - 1)
				vs.lastG = false
			case vs.lastG:
				list.ScrollTo(0)
				vs.lastG = false
			default:
				vs.lastG = true
			}
		case key.NameHome:
			list.ScrollTo(0)
			vs.lastG = false
		case key.NameEnd:
			list.ScrollTo(n - 1)
			vs.lastG = false
		case "Q":
			os.Exit(0)
		case key.NameEscape:
			vs.lastG = false
		}
		w.Invalidate()
	}
}

// -----------------------------------------------------------------------------
// Blocks
// -----------------------------------------------------------------------------

type span struct {
	text   string
	bold   bool
	italic bool
	code   bool
	link   string
}

type block interface {
	layout(gtx C, th *material.Theme) D
}

type headingBlock struct {
	level int
	spans []span
	state richtext.InteractiveText
}

func (h *headingBlock) layout(gtx C, th *material.Theme) D {
	sizes := []unit.Sp{30, 24, 20, 18, 16, 15}
	lv := h.level
	if lv < 1 {
		lv = 1
	}
	if lv > 6 {
		lv = 6
	}
	top := unit.Dp(16)
	if lv == 1 {
		top = unit.Dp(20)
	}
	return layout.Inset{Top: top, Bottom: unit.Dp(6)}.Layout(gtx, func(gtx C) D {
		styled := make([]span, len(h.spans))
		copy(styled, h.spans)
		for i := range styled {
			styled[i].bold = true
		}
		return renderInline(gtx, th, styled, sizes[lv-1], &h.state)
	})
}

type paraBlock struct {
	spans []span
	state richtext.InteractiveText
}

func (p *paraBlock) layout(gtx C, th *material.Theme) D {
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx C) D {
		return renderInline(gtx, th, p.spans, unit.Sp(15), &p.state)
	})
}

type codeBlk struct {
	text string
}

func (c *codeBlk) layout(gtx C, th *material.Theme) D {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx C) D {
		macro := op.Record(gtx.Ops)
		dims := layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			lbl := material.Label(th, unit.Sp(13), strings.TrimRight(c.text, "\n"))
			lbl.Font.Typeface = "Go Mono"
			lbl.Color = colCodeFg
			return lbl.Layout(gtx)
		})
		call := macro.Stop()
		rect := image.Rectangle{Max: dims.Size}
		rr := clip.UniformRRect(rect, gtx.Dp(unit.Dp(4)))
		paint.FillShape(gtx.Ops, colCodeBg, rr.Op(gtx.Ops))
		call.Add(gtx.Ops)
		return dims
	})
}

type quoteBlk struct {
	spans []span
	state richtext.InteractiveText
}

func (q *quoteBlk) layout(gtx C, th *material.Theme) D {
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				barW := gtx.Dp(unit.Dp(3))
				macro := op.Record(gtx.Ops)
				gtx.Constraints.Min.X = barW
				gtx.Constraints.Max.X = barW
				// Render placeholder; height set by sibling.
				dims := D{Size: image.Pt(barW, gtx.Constraints.Min.Y)}
				_ = macro.Stop()
				return dims
			}),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Left: unit.Dp(12)}.Layout(gtx, func(gtx C) D {
					styled := make([]span, len(q.spans))
					copy(styled, q.spans)
					for i := range styled {
						styled[i].italic = true
					}
					macro := op.Record(gtx.Ops)
					dims := renderInline(gtx, th, styled, unit.Sp(15), &q.state)
					call := macro.Stop()
					// Bar to the left of the text.
					bar := image.Rect(-gtx.Dp(unit.Dp(12)), 0, -gtx.Dp(unit.Dp(9)), dims.Size.Y)
					paint.FillShape(gtx.Ops, colQuoteBar, clip.Rect(bar).Op())
					call.Add(gtx.Ops)
					return dims
				})
			}),
		)
	})
}

type listItemBlk struct {
	bullet string
	spans  []span
	depth  int
	state  richtext.InteractiveText
}

func (l *listItemBlk) layout(gtx C, th *material.Theme) D {
	return layout.Inset{
		Top: unit.Dp(2), Bottom: unit.Dp(2),
		Left: unit.Dp(float32(16 * (l.depth + 1))),
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				lbl := material.Label(th, unit.Sp(15), l.bullet+"  ")
				lbl.Color = colMuted
				return lbl.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx C) D {
				return renderInline(gtx, th, l.spans, unit.Sp(15), &l.state)
			}),
		)
	})
}

type hrule struct{}

func (h *hrule) layout(gtx C, th *material.Theme) D {
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx C) D {
		h1 := gtx.Dp(unit.Dp(1))
		rect := image.Rect(0, 0, gtx.Constraints.Max.X, h1)
		paint.FillShape(gtx.Ops, colRule, clip.Rect(rect).Op())
		return D{Size: image.Pt(gtx.Constraints.Max.X, h1)}
	})
}

// -----------------------------------------------------------------------------
// Inline rendering
// -----------------------------------------------------------------------------

func renderInline(gtx C, th *material.Theme, spans []span, size unit.Sp, state *richtext.InteractiveText) D {
	var rspans []richtext.SpanStyle
	for _, s := range spans {
		st := richtext.SpanStyle{
			Content: s.text,
			Size:    size,
			Color:   colFg,
		}
		if s.code {
			st.Font.Typeface = "Go Mono"
			st.Color = colCodeFg
		}
		if s.bold {
			st.Font.Weight = font.Bold
		}
		if s.italic {
			st.Font.Style = font.Italic
		}
		if s.link != "" {
			st.Color = colLink
			st.Interactive = true
		}
		rspans = append(rspans, st)
	}
	return richtext.Text(state, th.Shaper, rspans...).Layout(gtx)
}

// -----------------------------------------------------------------------------
// Markdown parsing → block list
// -----------------------------------------------------------------------------

func parseMarkdown(src []byte) []block {
	md := goldmark.New(goldmark.WithExtensions(gmext.GFM))
	doc := md.Parser().Parse(gmtext.NewReader(src))
	var blocks []block
	walkBlocks(doc, src, &blocks, 0)
	return blocks
}

func walkBlocks(parent ast.Node, src []byte, blocks *[]block, depth int) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.Heading:
			*blocks = append(*blocks, &headingBlock{level: n.Level, spans: inlineSpans(n, src)})
		case *ast.Paragraph:
			*blocks = append(*blocks, &paraBlock{spans: inlineSpans(n, src)})
		case *ast.FencedCodeBlock:
			*blocks = append(*blocks, &codeBlk{text: nodeText(n, src)})
		case *ast.CodeBlock:
			*blocks = append(*blocks, &codeBlk{text: nodeText(n, src)})
		case *ast.ThematicBreak:
			*blocks = append(*blocks, &hrule{})
		case *ast.Blockquote:
			for p := n.FirstChild(); p != nil; p = p.NextSibling() {
				if pp, ok := p.(*ast.Paragraph); ok {
					*blocks = append(*blocks, &quoteBlk{spans: inlineSpans(pp, src)})
				}
			}
		case *ast.List:
			idx := 1
			for li := n.FirstChild(); li != nil; li = li.NextSibling() {
				item, ok := li.(*ast.ListItem)
				if !ok {
					continue
				}
				var bullet string
				if n.IsOrdered() {
					bullet = strconv.Itoa(idx) + "."
					idx++
				} else {
					bullet = "•"
				}
				first := true
				for ic := item.FirstChild(); ic != nil; ic = ic.NextSibling() {
					switch in := ic.(type) {
					case *ast.Paragraph, *ast.TextBlock:
						b := bullet
						if !first {
							b = ""
						}
						first = false
						*blocks = append(*blocks, &listItemBlk{
							bullet: b,
							spans:  inlineSpans(in, src),
							depth:  depth,
						})
					case *ast.List:
						walkBlocks(item, src, blocks, depth+1)
					}
				}
			}
		default:
			// Recurse into unknown container nodes.
			if c.HasChildren() {
				walkBlocks(c, src, blocks, depth)
			}
		}
	}
}

func inlineSpans(parent ast.Node, src []byte) []span {
	var out []span
	var walk func(n ast.Node, bold, italic, code bool, link string)
	walk = func(n ast.Node, bold, italic, code bool, link string) {
		switch t := n.(type) {
		case *ast.Text:
			seg := string(t.Segment.Value(src))
			if seg != "" {
				out = append(out, span{text: seg, bold: bold, italic: italic, code: code, link: link})
			}
			if t.SoftLineBreak() {
				out = append(out, span{text: " "})
			}
			if t.HardLineBreak() {
				out = append(out, span{text: "\n"})
			}
			return
		case *ast.String:
			out = append(out, span{text: string(t.Value), bold: bold, italic: italic, code: code, link: link})
			return
		case *ast.CodeSpan:
			var sb strings.Builder
			for c := t.FirstChild(); c != nil; c = c.NextSibling() {
				if tn, ok := c.(*ast.Text); ok {
					sb.Write(tn.Segment.Value(src))
				}
			}
			out = append(out, span{text: sb.String(), code: true, link: link})
			return
		case *ast.Emphasis:
			if t.Level >= 2 {
				bold = true
			} else {
				italic = true
			}
		case *ast.Link:
			link = string(t.Destination)
		case *ast.AutoLink:
			url := string(t.URL(src))
			out = append(out, span{text: url, link: url})
			return
		case *ast.Image:
			out = append(out, span{text: "[image: " + string(t.Destination) + "]", italic: true})
			return
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c, bold, italic, code, link)
		}
	}
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		walk(c, false, false, false, "")
	}
	return out
}

func nodeText(n ast.Node, src []byte) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		sb.Write(line.Value(src))
	}
	return sb.String()
}

// -----------------------------------------------------------------------------
// Settings overlay
// -----------------------------------------------------------------------------

const desktopFileName = "mdviewer.desktop"

var mdMimeTypes = []string{"text/markdown", "text/x-markdown"}

type settingsState struct {
	open   bool
	toggle widget.Bool
	msg    string
	msgErr bool
}

func layoutSettings(gtx C, th *material.Theme, st *settingsState) D {
	if st.toggle.Update(gtx) {
		var err error
		if st.toggle.Value {
			err = enableDefaultViewer()
		} else {
			err = disableDefaultViewer()
		}
		if err != nil {
			st.msg = err.Error()
			st.msgErr = true
			st.toggle.Value = isDefaultViewer()
		} else {
			st.msgErr = false
			if st.toggle.Value {
				st.msg = "Registered as default .md viewer."
			} else {
				st.msg = "No longer the default .md viewer."
			}
		}
	}

	// Scrim that also captures clicks behind the panel.
	scrimRect := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	paint.ColorOp{Color: colScrim}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	scrimRect.Pop()

	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = gtx.Dp(unit.Dp(440))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		macro := op.Record(gtx.Ops)
		dims := layout.UniformInset(unit.Dp(22)).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceEnd}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					title := material.Label(th, unit.Sp(20), "Settings")
					title.Font.Weight = font.Bold
					return title.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(material.Label(th, unit.Sp(15), "Default viewer for .md files").Layout),
								layout.Rigid(func(gtx C) D {
									sub := material.Label(th, unit.Sp(12), "Registers a desktop entry and updates xdg-mime.")
									sub.Color = colMuted
									return sub.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(material.Switch(th, &st.toggle, "default md viewer").Layout),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
				layout.Rigid(func(gtx C) D {
					if st.msg == "" {
						return D{}
					}
					lbl := material.Label(th, unit.Sp(13), st.msg)
					if st.msgErr {
						lbl.Color = colErr
					} else {
						lbl.Color = colOk
					}
					return lbl.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
				layout.Rigid(func(gtx C) D {
					hint := material.Label(th, unit.Sp(12), "press , or Esc to close")
					hint.Color = colMuted
					return hint.Layout(gtx)
				}),
			)
		})
		call := macro.Stop()
		rect := image.Rectangle{Max: dims.Size}
		rr := clip.UniformRRect(rect, gtx.Dp(unit.Dp(10)))
		paint.FillShape(gtx.Ops, colPanel, rr.Op(gtx.Ops))
		call.Add(gtx.Ops)
		return dims
	})
}

func desktopDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "applications")
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")
}

func desktopFilePath() string {
	return filepath.Join(desktopDir(), desktopFileName)
}

func mimeappsPath() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "mimeapps.list")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "mimeapps.list")
}

func isDefaultViewer() bool {
	if _, err := os.Stat(desktopFilePath()); err != nil {
		return false
	}
	for _, mt := range mdMimeTypes {
		out, err := exec.Command("xdg-mime", "query", "default", mt).Output()
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(out)) == desktopFileName {
			return true
		}
	}
	return false
}

func enableDefaultViewer() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	dir := desktopDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=mdviewer
GenericName=Markdown Viewer
Comment=Wayland-native Markdown viewer
Exec=%s %%f
Terminal=false
NoDisplay=false
MimeType=text/markdown;text/x-markdown;
Categories=Utility;Viewer;TextTools;
`, exe)
	if err := os.WriteFile(desktopFilePath(), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write desktop file: %w", err)
	}
	_ = exec.Command("update-desktop-database", dir).Run()
	for _, mt := range mdMimeTypes {
		if err := exec.Command("xdg-mime", "default", desktopFileName, mt).Run(); err != nil {
			return fmt.Errorf("xdg-mime default %s: %w", mt, err)
		}
	}
	return nil
}

func disableDefaultViewer() error {
	path := mimeappsPath()
	if data, err := os.ReadFile(path); err == nil {
		lines := strings.Split(string(data), "\n")
		kept := make([]string, 0, len(lines))
		for _, ln := range lines {
			drop := false
			for _, mt := range mdMimeTypes {
				if strings.HasPrefix(ln, mt+"=") && strings.Contains(ln, desktopFileName) {
					drop = true
					break
				}
			}
			if !drop {
				kept = append(kept, ln)
			}
		}
		if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
			return fmt.Errorf("update %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := os.Remove(desktopFilePath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove desktop file: %w", err)
	}
	_ = exec.Command("update-desktop-database", desktopDir()).Run()
	return nil
}
