package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

type testEPUBOpts struct {
	Title       string
	Authors     []string
	ISBN        string
	Publisher   string
	Subjects    []string
	Description string
	Language    string
	Series      string
	SeriesIndex string
	HasCover    bool
	Content     string
}

func defaultOpts() testEPUBOpts {
	return testEPUBOpts{
		Title:       "Test Book",
		Authors:     []string{"John Doe"},
		ISBN:        "9780134190440",
		Publisher:   "Test Publisher",
		Subjects:    []string{"Fiction", "Sci-Fi"},
		Description: "A test book description.",
		Language:    "en",
		HasCover:    true,
		Content:     "<p>This is the content of the test book. It has several words for counting purposes.</p>",
	}
}

func createTestEPUB(t *testing.T, dir string, name string, opts testEPUBOpts) string {
	t.Helper()
	path := filepath.Join(dir, name)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// mimetype (must be first, uncompressed)
	mw, _ := zw.Create("mimetype")
	_, _ = mw.Write([]byte("application/epub+zip"))

	// container.xml
	cw, _ := zw.Create("META-INF/container.xml")
	_, _ = fmt.Fprintf(cw, `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`)

	// Build OPF
	opf := buildOPF(opts)
	ow, _ := zw.Create("OEBPS/content.opf")
	_, _ = ow.Write([]byte(opf))

	// Content XHTML
	xw, _ := zw.Create("OEBPS/chapter1.xhtml")
	_, _ = fmt.Fprintf(xw, `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Chapter 1</title></head>
<body>%s</body>
</html>`, opts.Content)

	// Cover image (small PNG)
	if opts.HasCover {
		iw, _ := zw.Create("OEBPS/cover.png")
		img := image.NewRGBA(image.Rect(0, 0, 100, 150))
		for y := range 150 {
			for x := range 100 {
				img.Set(x, y, color.RGBA{R: 42, G: 42, B: 200, A: 255})
			}
		}
		_ = png.Encode(iw, img)
	}

	_ = zw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write test epub: %v", err)
	}
	return path
}

func buildOPF(opts testEPUBOpts) string {
	var b bytes.Buffer
	b.WriteString(`<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
  <metadata xmlns:opf="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:calibre="http://calibre.kovidgoyal.net/2009/metadata">
`)
	fmt.Fprintf(&b, "    <dc:title>%s</dc:title>\n", opts.Title)
	for _, a := range opts.Authors {
		fmt.Fprintf(&b, "    <dc:creator opf:role=\"aut\">%s</dc:creator>\n", a)
	}
	if opts.ISBN != "" {
		fmt.Fprintf(&b, "    <dc:identifier id=\"bookid\" opf:scheme=\"ISBN\">%s</dc:identifier>\n", opts.ISBN)
	} else {
		b.WriteString("    <dc:identifier id=\"bookid\">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier>\n")
	}
	if opts.Publisher != "" {
		fmt.Fprintf(&b, "    <dc:publisher>%s</dc:publisher>\n", opts.Publisher)
	}
	for _, s := range opts.Subjects {
		fmt.Fprintf(&b, "    <dc:subject>%s</dc:subject>\n", s)
	}
	if opts.Description != "" {
		fmt.Fprintf(&b, "    <dc:description>%s</dc:description>\n", opts.Description)
	}
	if opts.Language != "" {
		fmt.Fprintf(&b, "    <dc:language>%s</dc:language>\n", opts.Language)
	}
	if opts.Series != "" {
		fmt.Fprintf(&b, "    <meta name=\"calibre:series\" content=\"%s\"/>\n", opts.Series)
	}
	if opts.SeriesIndex != "" {
		fmt.Fprintf(&b, "    <meta name=\"calibre:series_index\" content=\"%s\"/>\n", opts.SeriesIndex)
	}
	if opts.HasCover {
		b.WriteString("    <meta name=\"cover\" content=\"cover-img\"/>\n")
	}
	b.WriteString("  </metadata>\n  <manifest>\n")
	b.WriteString("    <item id=\"ch1\" href=\"chapter1.xhtml\" media-type=\"application/xhtml+xml\"/>\n")
	if opts.HasCover {
		b.WriteString("    <item id=\"cover-img\" href=\"cover.png\" media-type=\"image/png\"/>\n")
	}
	b.WriteString("  </manifest>\n  <spine>\n")
	b.WriteString("    <itemref idref=\"ch1\"/>\n")
	b.WriteString("  </spine>\n</package>")
	return b.String()
}
