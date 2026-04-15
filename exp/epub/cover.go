package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
)

// ExtractCover extracts the cover image from an EPUB and writes it as a JPEG
// to destPath. Quality controls JPEG compression (1-100); values <= 0 default to 85.
func ExtractCover(epubPath string, destPath string, quality int) error {
	if quality <= 0 {
		quality = 85
	}
	data, err := os.ReadFile(epubPath) // #nosec G304 -- path is caller-controlled, not user input
	if err != nil {
		return fmt.Errorf("read epub: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	containerPath := findContainerOPFPath(zr)
	if containerPath == "" {
		return fmt.Errorf("no OPF path found")
	}

	coverPath := findCoverImagePath(zr, containerPath)
	if coverPath == "" {
		return fmt.Errorf("no cover image found")
	}

	var coverFile *zip.File
	for _, f := range zr.File {
		if f.Name == coverPath {
			coverFile = f
			break
		}
	}
	if coverFile == nil {
		return fmt.Errorf("cover file not found in archive: %s", coverPath)
	}

	rc, err := coverFile.Open()
	if err != nil {
		return fmt.Errorf("open cover: %w", err)
	}
	defer func() { _ = rc.Close() }()

	img, _, err := image.Decode(rc)
	if err != nil {
		return fmt.Errorf("decode cover image: %w", err)
	}

	out, err := os.Create(destPath) // #nosec G304 -- destPath is caller-controlled, not user input
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer func() { _ = out.Close() }()

	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encode jpeg: %w", err)
	}
	return nil
}

func findContainerOPFPath(zr *zip.Reader) string {
	for _, f := range zr.File {
		if f.Name == "META-INF/container.xml" {
			return readContainerOPFPath(f)
		}
	}
	return ""
}

func readContainerOPFPath(f *zip.File) string {
	rc, err := f.Open()
	if err != nil {
		return ""
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return ""
	}
	type container struct {
		Rootfiles struct {
			Rootfile struct {
				Path string `xml:"full-path,attr"`
			} `xml:"rootfile"`
		} `xml:"rootfiles"`
	}
	var c container
	if err := xmlDecode(data, &c); err != nil {
		return ""
	}
	return c.Rootfiles.Rootfile.Path
}
