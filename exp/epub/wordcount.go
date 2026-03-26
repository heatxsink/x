package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// CountWordsAndPages counts the total words across all spine content documents
// in the EPUB and calculates an estimated page count. Values of wordsPerPage <= 0
// default to 250.
func CountWordsAndPages(epubPath string, wordsPerPage int) (words int, pages int, err error) {
	if wordsPerPage <= 0 {
		wordsPerPage = 250
	}
	data, err := os.ReadFile(epubPath)
	if err != nil {
		return 0, 0, fmt.Errorf("read epub: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, 0, fmt.Errorf("open zip: %w", err)
	}

	containerPath := findContainerOPFPath(zr)
	if containerPath == "" {
		return 0, 0, fmt.Errorf("no OPF path found")
	}

	contentPaths := spineContentPaths(zr, containerPath)
	if len(contentPaths) == 0 {
		return 0, 0, nil
	}

	var totalWords int
	for _, cp := range contentPaths {
		w, err := countWordsInFile(zr, cp)
		if err != nil {
			continue
		}
		totalWords += w
	}

	pages = totalWords / wordsPerPage
	if totalWords%wordsPerPage > 0 && totalWords > 0 {
		pages++
	}
	return totalWords, pages, nil
}

func countWordsInFile(zr *zip.Reader, filePath string) (int, error) {
	var f *zip.File
	for _, zf := range zr.File {
		if zf.Name == filePath {
			f = zf
			break
		}
	}
	if f == nil {
		return 0, fmt.Errorf("file not found: %s", filePath)
	}
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return 0, err
	}

	text := stripXMLTags(string(data))
	return countWords(text), nil
}

func stripXMLTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			b.WriteRune(' ')
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func countWords(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				count++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return count
}
