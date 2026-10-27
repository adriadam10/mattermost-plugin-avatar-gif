package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"

	"golang.org/x/image/draw"
)

// Mattermost stores and serves every profile picture at this size.
const avatarSize = 128

// Larger canvases are rejected before decoding every frame, to bound memory.
const maxCanvasPixels = 2000 * 2000

var (
	errNotGIF        = errors.New("not a GIF")
	errNotAnimated   = errors.New("GIF has a single frame")
	errTooManyFrames = errors.New("too many frames")
	errTooLarge      = errors.New("canvas too large")
)

func isGIF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
}

// resizeGIF crops the animation to a centred square and scales it to the avatar size,
// keeping every frame and its delay. It also returns the first frame as a PNG.
func resizeGIF(data []byte, maxFrames int) (animated, firstFrame []byte, err error) {
	if !isGIF(data) {
		return nil, nil, errNotGIF
	}
	cfg, err := gif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	if cfg.Width*cfg.Height > maxCanvasPixels {
		return nil, nil, errTooLarge
	}
	src, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	if len(src.Image) < 2 {
		return nil, nil, errNotAnimated
	}
	if len(src.Image) > maxFrames {
		return nil, nil, errTooManyFrames
	}

	bounds := image.Rect(0, 0, src.Config.Width, src.Config.Height)
	crop := centredSquare(bounds)
	canvas := image.NewRGBA(bounds)
	scaled := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	out := &gif.GIF{LoopCount: src.LoopCount}
	var first image.Image

	for i, frame := range src.Image {
		var saved *image.RGBA
		if disposal(src, i) == gif.DisposalPrevious {
			saved = image.NewRGBA(bounds)
			draw.Draw(saved, bounds, canvas, image.Point{}, draw.Src)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		draw.Draw(scaled, scaled.Bounds(), image.Transparent, image.Point{}, draw.Src)
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), canvas, crop, draw.Over, nil)
		out.Image = append(out.Image, toPaletted(scaled, frame.Palette))
		out.Delay = append(out.Delay, src.Delay[i])
		// Every output frame is complete, so each one replaces the previous.
		out.Disposal = append(out.Disposal, gif.DisposalBackground)
		if i == 0 {
			first = image.Image(cloneRGBA(scaled))
		}

		switch disposal(src, i) {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = saved
		}
	}

	var gifBuf, pngBuf bytes.Buffer
	if err := gif.EncodeAll(&gifBuf, out); err != nil {
		return nil, nil, err
	}
	if err := png.Encode(&pngBuf, first); err != nil {
		return nil, nil, err
	}
	return gifBuf.Bytes(), pngBuf.Bytes(), nil
}

func disposal(g *gif.GIF, i int) byte {
	if i < len(g.Disposal) {
		return g.Disposal[i]
	}
	return gif.DisposalNone
}

func centredSquare(r image.Rectangle) image.Rectangle {
	side := min(r.Dx(), r.Dy())
	x := r.Min.X + (r.Dx()-side)/2
	y := r.Min.Y + (r.Dy()-side)/2
	return image.Rect(x, y, x+side, y+side)
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

// toPaletted maps the scaled frame back onto the frame's own palette, without dithering
// (dithering flickers between frames). Mostly transparent pixels become transparent.
func toPaletted(img *image.RGBA, pal color.Palette) *image.Paletted {
	pal = withTransparent(pal)
	transparent := uint8(len(pal) - 1)
	dst := image.NewPaletted(img.Bounds(), pal)
	cache := make(map[color.RGBA]uint8)
	for i := 0; i < len(img.Pix); i += 4 {
		c := color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
		if c.A < 128 {
			dst.Pix[i/4] = transparent
			continue
		}
		c.A = 255
		idx, ok := cache[c]
		if !ok {
			idx = uint8(pal[:transparent].Index(c))
			cache[c] = idx
		}
		dst.Pix[i/4] = idx
	}
	return dst
}

// withTransparent returns the opaque colours of pal followed by one transparent entry.
func withTransparent(pal color.Palette) color.Palette {
	opaque := make(color.Palette, 0, len(pal)+1)
	for _, c := range pal {
		if _, _, _, a := c.RGBA(); a == 0xffff {
			opaque = append(opaque, c)
		}
	}
	if len(opaque) == 0 {
		opaque = append(opaque, color.Black)
	}
	if len(opaque) > 255 {
		opaque = opaque[:255]
	}
	return append(opaque, color.RGBA{})
}

// nudgePixel returns the PNG with one colour channel of one pixel changed by one,
// so Mattermost sees a different picture that looks the same.
func nudgePixel(data []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	rgba.Pix[0] ^= 1
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
