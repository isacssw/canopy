package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type semanticColor struct {
	name  string
	value ansi.RGBColor
}

func remapANSIForTheme(input string, theme Theme) string {
	var out strings.Builder
	parser := ansi.NewParser()
	parser.SetHandler(ansi.Handler{
		Print: func(r rune) {
			out.WriteRune(r)
		},
		Execute: func(b byte) {
			out.WriteByte(b)
		},
		HandleCsi: func(cmd ansi.Cmd, params ansi.Params) {
			if cmd.Final() == 'm' && cmd.Prefix() == 0 && cmd.Intermediate() == 0 {
				out.WriteString(remapSGR(params, theme))
				return
			}
			writeCSI(&out, cmd, params)
		},
		HandleEsc: func(cmd ansi.Cmd) {
			writeESC(&out, cmd)
		},
		HandleDcs: func(cmd ansi.Cmd, params ansi.Params, data []byte) {
			writeDCS(&out, cmd, params, data)
		},
		HandleOsc: func(cmd int, data []byte) {
			out.WriteString("\x1b]")
			out.WriteString(strconv.Itoa(cmd))
			if len(data) > 0 {
				out.WriteByte(';')
				out.Write(data)
			}
			out.WriteString("\x1b\\")
		},
		HandlePm: func(data []byte) {
			writeStringSeq(&out, "\x1b^", data)
		},
		HandleApc: func(data []byte) {
			writeStringSeq(&out, "\x1b_", data)
		},
		HandleSos: func(data []byte) {
			writeStringSeq(&out, "\x1bX", data)
		},
	})
	parser.Parse([]byte(input))
	return out.String()
}

func remapSGR(params ansi.Params, theme Theme) string {
	if len(params) == 0 {
		return "\x1b[0m"
	}

	values := make([]int, 0, len(params))
	params.ForEach(0, func(_ int, param int, _ bool) {
		values = append(values, param)
	})

	parts := make([]string, 0, len(values))
	for i := 0; i < len(values); i++ {
		p := values[i]
		switch {
		case p == 38 || p == 58:
			var c color.Color
			read := ansi.ReadStyleColor(params[i+1:], &c)
			if read <= 0 {
				parts = append(parts, strconv.Itoa(p))
				continue
			}
			rgb := nearestSemanticColor(c, theme)
			parts = append(parts, rgbSGRParams(p, rgb)...)
			i += read
		case p == 48:
			var c color.Color
			read := ansi.ReadStyleColor(params[i+1:], &c)
			if read <= 0 {
				parts = append(parts, strconv.Itoa(p))
				continue
			}
			parts = append(parts, "49")
			i += read
		case p == 39:
			parts = append(parts, rgbSGRParams(38, themeColorToRGB(theme.Text))...)
		case p == 49:
			parts = append(parts, "49")
		case 30 <= p && p <= 37:
			parts = append(parts, rgbSGRParams(38, nearestSemanticColor(ansi.BasicColor(p-30), theme))...)
		case 90 <= p && p <= 97:
			parts = append(parts, rgbSGRParams(38, nearestSemanticColor(ansi.BasicColor(p-90+8), theme))...)
		case 40 <= p && p <= 47:
			parts = append(parts, "49")
		case 100 <= p && p <= 107:
			parts = append(parts, "49")
		default:
			parts = append(parts, strconv.Itoa(p))
		}
	}

	return "\x1b[" + strings.Join(parts, ";") + "m"
}

func nearestSemanticColor(c color.Color, theme Theme) ansi.RGBColor {
	target := normalizeColor(c)
	if neutral, ok := neutralSemanticColor(target, theme); ok {
		return neutral
	}

	candidates := []semanticColor{
		{name: "text", value: themeColorToRGB(theme.Text)},
		{name: "muted", value: themeColorToRGB(theme.Muted)},
		{name: "accent", value: themeColorToRGB(theme.Accent)},
		{name: "green", value: themeColorToRGB(theme.Green)},
		{name: "yellow", value: themeColorToRGB(theme.Yellow)},
		{name: "red", value: themeColorToRGB(theme.Red)},
		{name: "purple", value: themeColorToRGB(theme.Purple)},
	}

	best := candidates[0].value
	bestDist := colorDistance(target, best)
	for _, candidate := range candidates[1:] {
		if dist := colorDistance(target, candidate.value); dist < bestDist {
			bestDist = dist
			best = candidate.value
		}
	}
	return best
}

func neutralSemanticColor(c ansi.RGBColor, theme Theme) (ansi.RGBColor, bool) {
	maxCh := maxRGB(c.R, c.G, c.B)
	minCh := minRGB(c.R, c.G, c.B)
	if int(maxCh)-int(minCh) > 18 {
		return ansi.RGBColor{}, false
	}

	luma := int(c.R)*299 + int(c.G)*587 + int(c.B)*114
	luma /= 1000

	switch {
	case luma >= 200:
		return themeColorToRGB(theme.Text), true
	case luma <= 90:
		return themeColorToRGB(theme.Text), true
	default:
		return themeColorToRGB(theme.Muted), true
	}
}

func rgbSGRParams(prefix int, c ansi.RGBColor) []string {
	return []string{
		strconv.Itoa(prefix),
		"2",
		strconv.Itoa(int(c.R)),
		strconv.Itoa(int(c.G)),
		strconv.Itoa(int(c.B)),
	}
}

func themeColorToRGB(c lipgloss.Color) ansi.RGBColor {
	raw := strings.TrimPrefix(string(c), "#")
	if len(raw) != 6 {
		return ansi.RGBColor{}
	}

	value, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return ansi.RGBColor{}
	}

	return ansi.RGBColor{
		R: uint8(value >> 16),
		G: uint8((value >> 8) & 0xff),
		B: uint8(value & 0xff),
	}
}

func normalizeColor(c color.Color) ansi.RGBColor {
	r, g, b, _ := c.RGBA()
	return ansi.RGBColor{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
	}
}

func colorDistance(a, b ansi.RGBColor) int64 {
	dr := int64(a.R) - int64(b.R)
	dg := int64(a.G) - int64(b.G)
	db := int64(a.B) - int64(b.B)
	return dr*dr + dg*dg + db*db
}

func maxRGB(a, b, c uint8) uint8 {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}

func minRGB(a, b, c uint8) uint8 {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}

func writeCSI(out *strings.Builder, cmd ansi.Cmd, params ansi.Params) {
	out.WriteString("\x1b[")
	if prefix := cmd.Prefix(); prefix != 0 {
		out.WriteByte(prefix)
	}
	out.WriteString(serializeParams(params))
	if intermediate := cmd.Intermediate(); intermediate != 0 {
		out.WriteByte(intermediate)
	}
	out.WriteByte(cmd.Final())
}

func writeESC(out *strings.Builder, cmd ansi.Cmd) {
	out.WriteByte('\x1b')
	if intermediate := cmd.Intermediate(); intermediate != 0 {
		out.WriteByte(intermediate)
	}
	out.WriteByte(cmd.Final())
}

func writeDCS(out *strings.Builder, cmd ansi.Cmd, params ansi.Params, data []byte) {
	out.WriteString("\x1bP")
	if prefix := cmd.Prefix(); prefix != 0 {
		out.WriteByte(prefix)
	}
	out.WriteString(serializeParams(params))
	if intermediate := cmd.Intermediate(); intermediate != 0 {
		out.WriteByte(intermediate)
	}
	out.WriteByte(cmd.Final())
	out.Write(data)
	out.WriteString("\x1b\\")
}

func writeStringSeq(out *strings.Builder, prefix string, data []byte) {
	out.WriteString(prefix)
	out.Write(data)
	out.WriteString("\x1b\\")
}

func serializeParams(params ansi.Params) string {
	var out strings.Builder
	first := true
	prevHasMore := false
	params.ForEach(0, func(_ int, param int, hasMore bool) {
		if !first {
			if prevHasMore {
				out.WriteByte(':')
			} else {
				out.WriteByte(';')
			}
		}
		out.WriteString(fmt.Sprintf("%d", param))
		first = false
		prevHasMore = hasMore
	})
	return out.String()
}
