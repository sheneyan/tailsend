#!/usr/bin/env python3
"""Rasterize OS device marks (no font glyphs) for desktop + Android."""

from pathlib import Path

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
DESKTOP = ROOT / "desktop" / "frontend" / "src" / "assets" / "os"
ANDROID = ROOT / "android" / "app" / "src" / "main" / "res" / "drawable"

# Same palette as desktop osColor().
COLORS = {
    "mac": (107, 114, 128, 255),
    "ios": (107, 114, 128, 255),
    "win": (59, 130, 246, 255),
    "android": (34, 197, 94, 255),
    "linux": (245, 158, 11, 255),
    "other": (91, 141, 239, 255),
}

SIZE = 192
SCALE = 4
WHITE = (255, 255, 255, 255)
INK = (0, 0, 0, 70)


def canvas():
    s = SIZE * SCALE
    img = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    return img, ImageDraw.Draw(img), s


def bg(draw, s, color):
    m = int(s * 0.02)
    r = int(s * 0.28)
    draw.rounded_rectangle((m, m, s - m, s - m), radius=r, fill=color)


def u(s, x, y):
    # SVG 24x24 padded inside the rounded square.
    pad = s * 0.22
    inner = s - 2 * pad
    return pad + x / 24 * inner, pad + y / 24 * inner


def box(s, x, y, w, h):
    x0, y0 = u(s, x, y)
    x1, y1 = u(s, x + w, y + h)
    if x1 < x0:
        x0, x1 = x1, x0
    if y1 < y0:
        y0, y1 = y1, y0
    if y1 - y0 < 2:
        y1 = y0 + 2
    if x1 - x0 < 2:
        x1 = x0 + 2
    return x0, y0, x1, y1


def rrect(draw, xy, radius, **kwargs):
    x0, y0, x1, y1 = xy
    r = min(radius, (x1 - x0) / 2, (y1 - y0) / 2)
    draw.rounded_rectangle((x0, y0, x1, y1), radius=max(0, r), **kwargs)


def finish(img):
    return img.resize((SIZE, SIZE), Image.Resampling.LANCZOS)


def draw_mac(color):
    img, d, s = canvas()
    bg(d, s, color)
    rrect(d, box(s, 3, 3, 18, 13), s * 0.04, fill=WHITE)
    rrect(d, box(s, 5, 5, 14, 9), s * 0.015, outline=INK, width=max(1, s // 80))
    d.polygon(
        [u(s, 10, 16), u(s, 14, 16), u(s, 15.5, 19.5), u(s, 8.5, 19.5)],
        fill=WHITE,
    )
    return finish(img)


def draw_win(color):
    img, d, s = canvas()
    bg(d, s, color)
    rrect(d, box(s, 3, 4, 18, 16), s * 0.03, fill=WHITE)
    rrect(d, box(s, 3, 4, 18, 4), s * 0.03, fill=INK)
    rrect(d, box(s, 5.5, 10, 5.5, 4), s * 0.012, fill=INK)
    rrect(d, box(s, 13, 10, 5.5, 4), s * 0.012, fill=INK)
    return finish(img)


def draw_linux(color):
    img, d, s = canvas()
    bg(d, s, color)
    d.polygon(
        [u(s, 12, 2.5), u(s, 20, 7), u(s, 20, 17), u(s, 12, 21.5), u(s, 4, 17), u(s, 4, 7)],
        fill=WHITE,
    )
    r = max(2, s // 55)
    d.ellipse(
        [u(s, 9.5, 10)[0] - r, u(s, 9.5, 10)[1] - r, u(s, 9.5, 10)[0] + r, u(s, 9.5, 10)[1] + r],
        fill=INK,
    )
    d.ellipse(
        [u(s, 14.5, 10)[0] - r, u(s, 14.5, 10)[1] - r, u(s, 14.5, 10)[0] + r, u(s, 14.5, 10)[1] + r],
        fill=INK,
    )
    d.arc(box(s, 8, 11, 8, 6), 20, 160, fill=INK, width=max(2, s // 70))
    return finish(img)


def draw_android(color):
    img, d, s = canvas()
    bg(d, s, color)
    rrect(d, box(s, 7, 2, 10, 20), s * 0.045, fill=WHITE)
    rrect(d, box(s, 9, 4.5, 6, 12), s * 0.012, fill=INK)
    rrect(d, box(s, 10, 18.2, 4, 1.4), s * 0.012, fill=INK)
    return finish(img)


def draw_ios(color):
    img, d, s = canvas()
    bg(d, s, color)
    rrect(d, box(s, 7, 2, 10, 20), s * 0.06, fill=WHITE)
    rrect(d, box(s, 9, 5.2, 6, 11.5), s * 0.015, fill=INK)
    rrect(d, box(s, 10.2, 3.3, 3.6, 1.6), s * 0.012, fill=INK)
    return finish(img)


def draw_other(color):
    img, d, s = canvas()
    bg(d, s, color)
    x0, y0, x1, y1 = box(s, 3, 3, 18, 18)
    d.ellipse((x0, y0, x1, y1), fill=WHITE)
    cx, cy = u(s, 12, 12)
    r = (x1 - x0) * 0.16
    d.ellipse((cx - r, cy - r, cx + r, cy + r), fill=INK)
    return finish(img)


DRAW = {
    "mac": draw_mac,
    "win": draw_win,
    "linux": draw_linux,
    "android": draw_android,
    "ios": draw_ios,
    "other": draw_other,
}


def main():
    DESKTOP.mkdir(parents=True, exist_ok=True)
    ANDROID.mkdir(parents=True, exist_ok=True)
    for name, fn in DRAW.items():
        img = fn(COLORS[name])
        desktop = DESKTOP / f"{name}.png"
        android = ANDROID / f"os_{name}.png"
        img.save(desktop, "PNG")
        img.save(android, "PNG")
        print(desktop)
        print(android)


if __name__ == "__main__":
    main()
