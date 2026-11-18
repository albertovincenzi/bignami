#!/usr/bin/env python3
"""Turn a terminal frame (text with ANSI colors) into a crisp SVG screenshot.

Usage: ansi2svg.py frame.ansi out.svg [window-title]

Block and box-drawing characters are drawn as shapes, not text, so the picture looks the same
in every font. Everything else is text, placed on the cell grid.
"""
import html
import re
import sys

CW, CH, FS, PAD, BAR = 9.0, 20.0, 15, 22, 40
BG, FG = "#0F172A", "#E2E8F0"
BASIC = ["#1e293b", "#ef4444", "#22c55e", "#f59e0b", "#3b82f6", "#a855f7", "#06b6d4", "#e2e8f0",
         "#64748b", "#f87171", "#4ade80", "#fbbf24", "#60a5fa", "#c084fc", "#22d3ee", "#f8fafc"]
SGR = re.compile(r"\x1b\[([0-9;]*)([A-Za-z])")


def color256(n):
    if n < 16:
        return BASIC[n]
    if n >= 232:
        v = 8 + (n - 232) * 10
        return "#%02x%02x%02x" % (v, v, v)
    n -= 16
    lv = [0, 95, 135, 175, 215, 255]
    return "#%02x%02x%02x" % (lv[n // 36], lv[(n // 6) % 6], lv[n % 6])


def parse(text):
    """Return rows of (char, fg, bg, bold)."""
    rows, fg, bg, bold = [], None, None, False
    for line in text.split("\n"):
        row, pos = [], 0
        for m in SGR.finditer(line):
            for ch in line[pos:m.start()]:
                row.append((ch, fg, bg, bold))
            pos = m.end()
            if m.group(2) != "m":
                continue
            codes = [int(c) if c else 0 for c in m.group(1).split(";")]
            i = 0
            while i < len(codes):
                c = codes[i]
                if c == 0:
                    fg, bg, bold = None, None, False
                elif c == 1:
                    bold = True
                elif c == 22:
                    bold = False
                elif 30 <= c <= 37:
                    fg = BASIC[c - 30]
                elif 90 <= c <= 97:
                    fg = BASIC[c - 90 + 8]
                elif 40 <= c <= 47:
                    bg = BASIC[c - 40]
                elif 100 <= c <= 107:
                    bg = BASIC[c - 100 + 8]
                elif c == 39:
                    fg = None
                elif c == 49:
                    bg = None
                elif c in (38, 48) and i + 1 < len(codes):
                    if codes[i + 1] == 2 and i + 4 < len(codes):
                        col = "#%02x%02x%02x" % tuple(codes[i + 2:i + 5])
                        i += 4
                    elif codes[i + 1] == 5 and i + 2 < len(codes):
                        col = color256(codes[i + 2])
                        i += 2
                    else:
                        col = None
                    if c == 38:
                        fg = col
                    else:
                        bg = col
                i += 1
        for ch in line[pos:]:
            row.append((ch, fg, bg, bold))
        rows.append(row)
    return rows


BLOCKS = {  # char: (x0, y0, x1, y1) as fractions of the cell
    "█": (0, 0, 1, 1), "▀": (0, 0, 1, .5), "▄": (0, .5, 1, 1), "▌": (0, 0, .5, 1), "▐": (.5, 0, 1, 1),
}
SHADES = {"░": .25, "▒": .5, "▓": .75}


def box(ch, x, y, color):
    cx, cy = x + CW / 2, y + CH / 2
    r, x1, y1 = 5, x + CW, y + CH

    def p(d, w):
        return f'<path d="{d}" stroke="{color}" stroke-width="{w}" fill="none" stroke-linecap="square"/>'
    light = {
        "│": f"M{cx} {y} V{y1}", "─": f"M{x} {cy} H{x1}",
        "╭": f"M{x1} {cy} H{cx + r} Q{cx} {cy} {cx} {cy + r} V{y1}",
        "╮": f"M{x} {cy} H{cx - r} Q{cx} {cy} {cx} {cy + r} V{y1}",
        "╰": f"M{x1} {cy} H{cx + r} Q{cx} {cy} {cx} {cy - r} V{y}",
        "╯": f"M{x} {cy} H{cx - r} Q{cx} {cy} {cx} {cy - r} V{y}",
    }
    heavy = {
        "┃": f"M{cx} {y} V{y1}", "━": f"M{x} {cy} H{x1}",
        "┏": f"M{x1} {cy} H{cx} V{y1}", "┓": f"M{x} {cy} H{cx} V{y1}",
        "┗": f"M{x1} {cy} H{cx} V{y}", "┛": f"M{x} {cy} H{cx} V{y}",
    }
    if ch in light:
        return p(light[ch], 1.3)
    if ch in heavy:
        return p(heavy[ch], 3)
    return None


def render(rows, title):
    cols = max((len(r) for r in rows), default=1)
    while rows and not rows[-1]:
        rows.pop()
    w, h = cols * CW + 2 * PAD, len(rows) * CH + 2 * PAD + BAR
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{w:.0f}" height="{h:.0f}" viewBox="0 0 {w:.0f} {h:.0f}" role="img" aria-label="{html.escape(title)}">',
           f'<rect width="{w:.0f}" height="{h:.0f}" rx="14" fill="{BG}"/>',
           f'<rect x=".5" y=".5" width="{w - 1:.0f}" height="{h - 1:.0f}" rx="13.5" fill="none" stroke="#334155"/>']
    for i, c in enumerate(("#ef4444", "#f59e0b", "#22c55e")):
        out.append(f'<circle cx="{PAD + 7 + i * 20}" cy="{BAR / 2 + 2}" r="6" fill="{c}"/>')
    out.append(f'<text x="{w / 2:.0f}" y="{BAR / 2 + 7}" text-anchor="middle" font-family="Menlo,SFMono-Regular,Consolas,\'DejaVu Sans Mono\',monospace" font-size="13" fill="#94a3b8">{html.escape(title)}</text>')
    ox, oy = PAD, PAD + BAR
    fam = "Menlo,SFMono-Regular,Consolas,'DejaVu Sans Mono',monospace"
    for ri, row in enumerate(rows):
        y = oy + ri * CH
        # backgrounds, merged per run
        ci = 0
        while ci < len(row):
            bgc = row[ci][2]
            j = ci
            while j < len(row) and row[j][2] == bgc:
                j += 1
            if bgc:
                out.append(f'<rect x="{ox + ci * CW:.2f}" y="{y - .25:.2f}" width="{(j - ci) * CW:.2f}" height="{CH + .5}" fill="{bgc}" shape-rendering="crispEdges"/>')
            ci = j
        # shapes and text
        ci = 0
        while ci < len(row):
            ch, fg, bgc, bold = row[ci]
            x = ox + ci * CW
            color = fg or FG
            if ch in BLOCKS:
                a, b, c, d = BLOCKS[ch]
                out.append(f'<rect x="{x + a * CW - .25:.2f}" y="{y + b * CH - .25:.2f}" width="{(c - a) * CW + .5:.2f}" height="{(d - b) * CH + .5:.2f}" fill="{color}" shape-rendering="crispEdges"/>')
                ci += 1
            elif ch in SHADES:
                out.append(f'<rect x="{x:.2f}" y="{y:.2f}" width="{CW}" height="{CH}" fill="{color}" opacity="{SHADES[ch]}"/>')
                ci += 1
            elif (b := box(ch, x, y, color)):
                out.append(b)
                ci += 1
            elif ch == " ":
                ci += 1
            else:
                j = ci
                while j < len(row) and row[j][1:] == (fg, bgc, bold) and row[j][0] not in BLOCKS and row[j][0] not in SHADES and not box(row[j][0], 0, 0, "") and row[j][0] != " ":
                    j += 1
                text = "".join(c[0] for c in row[ci:j])
                weight = ' font-weight="700"' if bold else ""
                out.append(f'<text x="{x:.2f}" y="{y + CH * 0.74:.2f}" font-family="{fam}" font-size="{FS}" fill="{color}"{weight} textLength="{(j - ci) * CW:.2f}" lengthAdjust="spacingAndGlyphs" xml:space="preserve">{html.escape(text)}</text>')
                ci = j
    out.append("</svg>")
    return "\n".join(out)


if __name__ == "__main__":
    src, dst = sys.argv[1], sys.argv[2]
    title = sys.argv[3] if len(sys.argv) > 3 else "bignami"
    open(dst, "w").write(render(parse(open(src).read()), title))
