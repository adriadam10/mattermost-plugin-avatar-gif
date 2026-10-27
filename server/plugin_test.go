package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"net"
	"os"
	"testing"
)

var testPalette = color.Palette{color.Black, color.White, color.RGBA{255, 0, 0, 255}, color.RGBA{}}

// makeGIF builds an animation of the given size whose frames alternate between
// red and white, every frame a smaller rectangle in the middle after the first.
func makeGIF(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for i := range frames {
		r := image.Rect(0, 0, w, h)
		if i > 0 {
			r = image.Rect(w/4, h/4, w*3/4, h*3/4)
		}
		img := image.NewPaletted(r, testPalette)
		for j := range img.Pix {
			img.Pix[j] = uint8(2 - i%2)
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 5+i)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestResizeGIF(t *testing.T) {
	animated, first, err := resizeGIF(makeGIF(t, 300, 200, 3), 200)
	if err != nil {
		t.Fatal(err)
	}
	out, err := gif.DecodeAll(bytes.NewReader(animated))
	if err != nil {
		t.Fatal(err)
	}
	if out.Config.Width != avatarSize || out.Config.Height != avatarSize || len(out.Image) != 3 {
		t.Fatalf("got %dx%d with %d frames", out.Config.Width, out.Config.Height, len(out.Image))
	}
	if out.Delay[2] != 7 {
		t.Errorf("delay of frame 3 = %d, want 7", out.Delay[2])
	}
	// Frame 2 draws white over the middle; the corners keep the red of frame 1.
	if c := out.Image[1].At(64, 64); !sameRGB(c, color.White) {
		t.Errorf("middle of frame 2 = %v, want white", c)
	}
	if c := out.Image[1].At(2, 2); !sameRGB(c, color.RGBA{255, 0, 0, 255}) {
		t.Errorf("corner of frame 2 = %v, want red", c)
	}
	img, err := png.Decode(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != avatarSize || !sameRGB(img.At(64, 64), color.RGBA{255, 0, 0, 255}) {
		t.Errorf("first frame is %v, middle %v", img.Bounds(), img.At(64, 64))
	}
}

func TestResizeGIFDisposalBackground(t *testing.T) {
	src, err := gif.DecodeAll(bytes.NewReader(makeGIF(t, 128, 128, 2)))
	if err != nil {
		t.Fatal(err)
	}
	src.Disposal[0] = gif.DisposalBackground
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, src); err != nil {
		t.Fatal(err)
	}
	animated, _, err := resizeGIF(buf.Bytes(), 200)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := gif.DecodeAll(bytes.NewReader(animated))
	if _, _, _, a := out.Image[1].At(2, 2).RGBA(); a != 0 {
		t.Errorf("corner of frame 2 should be transparent once frame 1 is disposed, alpha %d", a)
	}
}

func TestResizeGIFRejects(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"not a GIF", []byte("\x89PNG\r\n"), errNotGIF},
		{"single frame", makeGIF(t, 50, 50, 1), errNotAnimated},
		{"too many frames", makeGIF(t, 20, 20, 5), errTooManyFrames},
		{"huge canvas", makeGIF(t, 3000, 2000, 2), errTooLarge},
	}
	for _, c := range cases {
		if _, _, err := resizeGIF(c.data, 4); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestNudgePixel(t *testing.T) {
	_, first, err := resizeGIF(makeGIF(t, 64, 64, 2), 200)
	if err != nil {
		t.Fatal(err)
	}
	nudged, err := nudgePixel(first)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(nudged, first) {
		t.Error("nudged PNG has the same bytes")
	}
}

func sameRGB(a, b color.Color) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return ar>>8 == br>>8 && ag>>8 == bg>>8 && ab>>8 == bb>>8
}

func TestIsPublic(t *testing.T) {
	for addr, want := range map[string]bool{
		"151.101.1.1":  true,
		"2a04:4e42::1": true,
		"127.0.0.1":    false,
		"10.1.2.3":     false,
		"192.168.77.9": false,
		"172.16.5.5":   false,
		"169.254.1.1":  false,
		"100.64.0.1":   false,
		"0.0.0.0":      false,
		"::1":          false,
		"fd00::1":      false,
	} {
		if got := isPublic(net.ParseIP(addr)); got != want {
			t.Errorf("isPublic(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestFetchRejectsBadURLs(t *testing.T) {
	for _, u := range []string{"http://example.com/a.gif", "ftp://example.com/a.gif", "https://127.0.0.1/a.gif", "https://localhost/a.gif"} {
		if _, err := fetchGIF(context.Background(), u, 1<<20); err == nil {
			t.Errorf("%s was fetched", u)
		}
	}
}

func TestPickOgImage(t *testing.T) {
	cases := map[string]string{
		// Giphy: .webp first, then .gif.
		`<meta property="og:image" content="https://media.example.com/a/giphy.webp"/>` +
			`<meta property="og:image" content="https://media.example.com/a/giphy.gif"/>`: "https://media.example.com/a/giphy.gif",
		// Tenor: other attributes first, and og:image:type must not match.
		`<meta class="dynamic" property="og:image" content="https://media.example.com/m/x.gif?a=1&amp;b=2">` +
			`<meta class="dynamic" property="og:image:type" content="image/gif">`: "https://media.example.com/m/x.gif?a=1&b=2",
		`<meta property="og:image" content="https://example.com/still.png">`: "https://example.com/still.png",
		`<title>no image</title>`: "",
	}
	for page, want := range cases {
		if got := pickOgImage(page); got != want {
			t.Errorf("pickOgImage(%s) = %q, want %q", page, got, want)
		}
	}
}

func TestDataDir(t *testing.T) {
	cases := []struct{ fileDir, pluginsDir, bundle, want string }{
		{"./data/", "./plugins", "/mattermost/plugins/" + manifestID, "/mattermost/data"},
		{"/srv/files", "./plugins", "/anything", "/srv/files"},
	}
	for _, c := range cases {
		got, err := dataDir(c.fileDir, c.pluginsDir, c.bundle)
		if err != nil || got != c.want {
			t.Errorf("dataDir(%q, %q, %q) = %q, %v; want %q", c.fileDir, c.pluginsDir, c.bundle, got, err, c.want)
		}
	}
	if _, err := dataDir("./data/", "./plugins", "/elsewhere/x"); err == nil {
		t.Error("an unrelated bundle path should fail")
	}
}

func TestConfiguration(t *testing.T) {
	c := rawConfiguration{MaxSizeMB: "8", MaxFrames: float64(50)}.parse()
	if c.MaxSizeMB != 8 || c.MaxFrames != 50 {
		t.Errorf("got %+v", c)
	}
	c = rawConfiguration{MaxSizeMB: "", MaxFrames: float64(-3)}.parse()
	if *c != *defaultConfiguration() {
		t.Errorf("invalid values should keep the defaults, got %+v", c)
	}
}

func TestManifestID(t *testing.T) {
	data, err := os.ReadFile("../plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ ID string }
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != manifestID {
		t.Errorf("plugin.json id %q, manifestID %q", m.ID, manifestID)
	}
}

func TestMessagesHaveBothLanguages(t *testing.T) {
	for key := range messages["en"] {
		if messages["es"][key] == "" {
			t.Errorf("missing Spanish text for %q", key)
		}
	}
	if len(messages["es"]) != len(messages["en"]) {
		t.Error("languages have different keys")
	}
}
