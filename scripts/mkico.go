// Generate a BMP-based .ico (go-winres cannot decode PNG-in-ICO).
// Usage: go run ./scripts/mkico.go desktop/build/appicon.png desktop/build/appicon.ico
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: mkico in.png out.ico\n")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	src, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sizes := []int{256, 48, 32, 16}
	var dibs [][]byte
	for _, s := range sizes {
		dibs = append(dibs, nrgbaToICODib(resizeNN(src, s, s)))
	}
	if err := os.WriteFile(os.Args[2], packICO(dibs, sizes), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resizeNN(src image.Image, w, h int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + x*sw/w
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func nrgbaToICODib(img *image.NRGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	xorStride := w * 4
	andStride := ((w + 31) / 32) * 4
	dibLen := 40 + xorStride*h + andStride*h
	dib := make([]byte, dibLen)
	binary.LittleEndian.PutUint32(dib[0:4], 40)
	binary.LittleEndian.PutUint32(dib[4:8], uint32(w))
	binary.LittleEndian.PutUint32(dib[8:12], uint32(h*2))
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 32)
	nrgba := img
	if img.Bounds().Min != image.Pt(0, 0) {
		nrgba = image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)
	}
	off := 40
	for y := h - 1; y >= 0; y-- {
		row := nrgba.Pix[y*nrgba.Stride : y*nrgba.Stride+w*4]
		for x := 0; x < w; x++ {
			r, g, b, a := row[x*4], row[x*4+1], row[x*4+2], row[x*4+3]
			dib[off] = b
			dib[off+1] = g
			dib[off+2] = r
			dib[off+3] = a
			off += 4
		}
	}
	return dib
}

func packICO(dibs [][]byte, sizes []int) []byte {
	count := len(dibs)
	hdr := 6 + 16*count
	total := hdr
	for _, d := range dibs {
		total += len(d)
	}
	out := make([]byte, total)
	binary.LittleEndian.PutUint16(out[2:4], 1)
	binary.LittleEndian.PutUint16(out[4:6], uint16(count))
	offset := hdr
	for i, d := range dibs {
		s := sizes[i]
		ew, eh := byte(s), byte(s)
		if s >= 256 {
			ew, eh = 0, 0
		}
		e := out[6+16*i : 6+16*(i+1)]
		e[0], e[1] = ew, eh
		binary.LittleEndian.PutUint16(e[4:6], 1)
		binary.LittleEndian.PutUint16(e[6:8], 32)
		binary.LittleEndian.PutUint32(e[8:12], uint32(len(d)))
		binary.LittleEndian.PutUint32(e[12:16], uint32(offset))
		copy(out[offset:], d)
		offset += len(d)
	}
	return out
}
