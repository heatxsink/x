package epub

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// rawEPUB writes an archive from explicit entries so a test can shape the
// OPF and file layout exactly. container.xml always points at opfPath.
func rawEPUB(t *testing.T, opfPath string, entries map[string][]byte) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("mimetype")
	_, _ = w.Write([]byte("application/epub+zip"))
	w, _ = zw.Create("META-INF/container.xml")
	_, _ = w.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="` + opfPath + `" media-type="application/oebps-package+xml"/></rootfiles>
</container>`))
	for name, data := range entries {
		w, _ = zw.Create(name)
		_, _ = w.Write(data)
	}
	_ = zw.Close()
	p := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(p, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write epub: %v", err)
	}
	return p
}

func opfDoc(metadata, manifest, guide string) []byte {
	return []byte(`<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>T</dc:title>
    <dc:identifier id="bookid">urn:uuid:1</dc:identifier>
` + metadata + `
  </metadata>
  <manifest>
    <item id="ch1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
` + manifest + `
  </manifest>
  <spine><itemref idref="ch1"/></spine>
` + guide + `
</package>`)
}

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		}
	}
	return img
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, testImage(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func gifBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.Encode(&b, testImage(w, h), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// extractAndMeasure runs ExtractCover and returns the decoded output size.
func extractAndMeasure(t *testing.T, epubPath string) (int, int) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "out.jpg")
	if err := ExtractCover(epubPath, dest, 85); err != nil {
		t.Fatalf("ExtractCover: %v", err)
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

func TestExtractCover_EPUB3Property(t *testing.T) {
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc("",
			`<item id="c" href="images/front.png" media-type="image/png" properties="svg cover-image"/>`, ""),
		"OEBPS/images/front.png": pngBytes(t, 40, 60),
	})
	if w, h := extractAndMeasure(t, p); w != 40 || h != 60 {
		t.Errorf("got %dx%d, want 40x60", w, h)
	}
}

func TestExtractCover_MetaCoverMissingMediaType(t *testing.T) {
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc(`<meta name="cover" content="cover-img"/>`,
			`<item id="cover-img" href="cover.png"/>`, ""),
		"OEBPS/cover.png": pngBytes(t, 30, 50),
	})
	if w, h := extractAndMeasure(t, p); w != 30 || h != 50 {
		t.Errorf("got %dx%d, want 30x50", w, h)
	}
}

func TestExtractCover_HrefDotDotAndPercentEncoding(t *testing.T) {
	p := rawEPUB(t, "OEBPS/text/content.opf", map[string][]byte{
		"OEBPS/text/content.opf": opfDoc(`<meta name="cover" content="cover-img"/>`,
			`<item id="cover-img" href="../images/cover%20art.png" media-type="image/png"/>`, ""),
		"OEBPS/images/cover art.png": pngBytes(t, 20, 30),
	})
	if w, h := extractAndMeasure(t, p); w != 20 || h != 30 {
		t.Errorf("got %dx%d, want 20x30", w, h)
	}
}

func TestExtractCover_MetaCoverPointsAtXHTMLPage(t *testing.T) {
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc(`<meta name="cover" content="cover-page"/>`,
			`<item id="cover-page" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
			 <item id="img1" href="images/c.png" media-type="image/png"/>`, ""),
		"OEBPS/text/cover.xhtml": []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body>
			<div><img src="../images/c.png" alt="cover"/></div></body></html>`),
		"OEBPS/images/c.png": pngBytes(t, 10, 15),
	})
	if w, h := extractAndMeasure(t, p); w != 10 || h != 15 {
		t.Errorf("got %dx%d, want 10x15", w, h)
	}
}

func TestExtractCover_SVGWrappedImageInCoverPage(t *testing.T) {
	p := rawEPUB(t, "content.opf", map[string][]byte{
		"content.opf": opfDoc("",
			`<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>
			 <item id="img1" href="c.png" media-type="image/png"/>`, ""),
		"cover.xhtml": []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body>
			<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
			<image width="10" height="15" xlink:href="c.png"/></svg></body></html>`),
		"c.png": pngBytes(t, 10, 15),
	})
	if w, h := extractAndMeasure(t, p); w != 10 || h != 15 {
		t.Errorf("got %dx%d, want 10x15", w, h)
	}
}

func TestExtractCover_GuideReference(t *testing.T) {
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc("",
			`<item id="x1" href="front.png" media-type="image/png"/>`,
			`<guide><reference type="cover" title="Cover" href="front.png"/></guide>`),
		"OEBPS/front.png": pngBytes(t, 12, 18),
	})
	if w, h := extractAndMeasure(t, p); w != 12 || h != 18 {
		t.Errorf("got %dx%d, want 12x18", w, h)
	}
}

func TestExtractCover_ManifestIDHeuristic(t *testing.T) {
	// No meta, no properties, no guide: only the manifest id says "cover".
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc("",
			`<item id="logo" href="images/logo.png" media-type="image/png"/>
			 <item id="cover" href="images/00001.jpeg" media-type="image/jpeg"/>`, ""),
		"OEBPS/images/logo.png":   pngBytes(t, 5, 5),
		"OEBPS/images/00001.jpeg": jpegBytes(t, 25, 35),
	})
	if w, h := extractAndMeasure(t, p); w != 25 || h != 35 {
		t.Errorf("got %dx%d, want 25x35", w, h)
	}
}

func TestExtractCover_BasenameContainsHeuristic(t *testing.T) {
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc("",
			`<item id="i1" href="images/logo.png" media-type="image/png"/>
			 <item id="i2" href="images/FrontCover_hi.png" media-type="image/png"/>`, ""),
		"OEBPS/images/logo.png":          pngBytes(t, 5, 5),
		"OEBPS/images/FrontCover_hi.png": pngBytes(t, 22, 33),
	})
	if w, h := extractAndMeasure(t, p); w != 22 || h != 33 {
		t.Errorf("got %dx%d, want 22x33", w, h)
	}
}

func TestExtractCover_HeuristicSkipsSVG(t *testing.T) {
	// The only thing named "cover" is an SVG, which we cannot decode. The
	// heuristic must not pick it, and with nothing else there is no cover.
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc("",
			`<item id="cover" href="cover.svg" media-type="image/svg+xml"/>`, ""),
		"OEBPS/cover.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
	})
	if err := ExtractCover(p, filepath.Join(t.TempDir(), "o.jpg"), 85); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestExtractCover_CaseInsensitiveZipEntry(t *testing.T) {
	p := rawEPUB(t, "OEBPS/content.opf", map[string][]byte{
		"OEBPS/content.opf": opfDoc(`<meta name="cover" content="cover-img"/>`,
			`<item id="cover-img" href="Images/Cover.png" media-type="image/png"/>`, ""),
		"OEBPS/images/cover.png": pngBytes(t, 7, 9),
	})
	if w, h := extractAndMeasure(t, p); w != 7 || h != 9 {
		t.Errorf("got %dx%d, want 7x9", w, h)
	}
}

func TestExtractCover_GIF(t *testing.T) {
	p := rawEPUB(t, "content.opf", map[string][]byte{
		"content.opf": opfDoc("",
			`<item id="c" href="cover.gif" media-type="image/gif" properties="cover-image"/>`, ""),
		"cover.gif": gifBytes(t, 16, 24),
	})
	if w, h := extractAndMeasure(t, p); w != 16 || h != 24 {
		t.Errorf("got %dx%d, want 16x24", w, h)
	}
}

func TestExtractCover_WebP(t *testing.T) {
	// A 3x2 lossless WebP produced by cwebp; x/image ships no encoder.
	webp := []byte{
		0x52, 0x49, 0x46, 0x46, 0x1c, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
		0x56, 0x50, 0x38, 0x4c, 0x0f, 0x00, 0x00, 0x00, 0x2f, 0x02, 0x40, 0x00,
		0x00, 0x07, 0x10, 0xfd, 0x8f, 0xfe, 0x07, 0x22, 0xa2, 0xff, 0x01, 0x00,
	}
	p := rawEPUB(t, "content.opf", map[string][]byte{
		"content.opf": opfDoc("",
			`<item id="c" href="cover.webp" media-type="image/webp" properties="cover-image"/>`, ""),
		"cover.webp": webp,
	})
	if w, h := extractAndMeasure(t, p); w != 3 || h != 2 {
		t.Errorf("got %dx%d, want 3x2", w, h)
	}
}

func TestExtractCover_ErrorNamesMissingEntry(t *testing.T) {
	p := rawEPUB(t, "content.opf", map[string][]byte{
		"content.opf": opfDoc("",
			`<item id="c" href="gone.png" media-type="image/png" properties="cover-image"/>`, ""),
	})
	err := ExtractCover(p, filepath.Join(t.TempDir(), "o.jpg"), 85)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "cover file not found in archive: gone.png" {
		t.Errorf("unexpected error text: %q", got)
	}
}

func TestResolveHref(t *testing.T) {
	cases := []struct{ dir, href, want string }{
		{"", "cover.jpg", "cover.jpg"},
		{"OEBPS", "cover.jpg", "OEBPS/cover.jpg"},
		{"OEBPS", "./cover.jpg", "OEBPS/cover.jpg"},
		{"OEBPS/text", "../images/c.jpg", "OEBPS/images/c.jpg"},
		{"OEBPS", "cover%20art.jpg", "OEBPS/cover art.jpg"},
		{"OEBPS", "cover.xhtml#top", "OEBPS/cover.xhtml"},
		{"OEBPS", "", ""},
	}
	for _, c := range cases {
		if got := resolveHref(c.dir, c.href); got != c.want {
			t.Errorf("resolveHref(%q, %q) = %q, want %q", c.dir, c.href, got, c.want)
		}
	}
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, testImage(w, h), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
