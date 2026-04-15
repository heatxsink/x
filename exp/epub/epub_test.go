package epub

import (
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Parse tests ---

func TestParse_AllFields(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	opts.Series = "Dune Chronicles"
	opts.SeriesIndex = "1"
	path := createTestEPUB(t, dir, "full.epub", opts)

	meta, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if meta.Title != "Test Book" {
		t.Errorf("title = %q", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0].Name != "John Doe" {
		t.Errorf("authors = %v", meta.Authors)
	}
	if meta.ISBN != "9780134190440" {
		t.Errorf("isbn = %q", meta.ISBN)
	}
	if meta.Publisher != "Test Publisher" {
		t.Errorf("publisher = %q", meta.Publisher)
	}
	if len(meta.Subjects) != 2 {
		t.Errorf("subjects = %v", meta.Subjects)
	}
	if meta.Description != "A test book description." {
		t.Errorf("description = %q", meta.Description)
	}
	if meta.Language != "en" {
		t.Errorf("language = %q", meta.Language)
	}
	if meta.Series != "Dune Chronicles" {
		t.Errorf("series = %q", meta.Series)
	}
	if meta.SeriesIndex != 1.0 {
		t.Errorf("series_index = %f", meta.SeriesIndex)
	}
}

func TestParse_MissingISBN(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	opts.ISBN = ""
	path := createTestEPUB(t, dir, "no-isbn.epub", opts)

	meta, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if meta.ISBN != "" {
		t.Errorf("expected empty ISBN, got %q", meta.ISBN)
	}
}

func TestParse_MissingAuthor(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	opts.Authors = nil
	path := createTestEPUB(t, dir, "no-author.epub", opts)

	meta, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(meta.Authors) != 0 {
		t.Errorf("expected no authors, got %v", meta.Authors)
	}
}

func TestParse_MultipleAuthors(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	opts.Authors = []string{"Alice Smith", "Bob Jones"}
	path := createTestEPUB(t, dir, "multi-author.epub", opts)

	meta, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(meta.Authors) != 2 {
		t.Fatalf("authors count = %d, want 2", len(meta.Authors))
	}
	if meta.Authors[0].Name != "Alice Smith" {
		t.Errorf("author 0 = %q", meta.Authors[0].Name)
	}
	if meta.Authors[1].Name != "Bob Jones" {
		t.Errorf("author 1 = %q", meta.Authors[1].Name)
	}
}

// --- ExtractCover tests ---

func TestExtractCover(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	epubPath := createTestEPUB(t, dir, "cover.epub", opts)
	destPath := filepath.Join(dir, "cover.jpg")

	if err := ExtractCover(epubPath, destPath, 85); err != nil {
		t.Fatalf("extract cover: %v", err)
	}

	f, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("open cover: %v", err)
	}
	defer func() { _ = f.Close() }()

	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decode jpeg: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 150 {
		t.Errorf("cover size = %dx%d, want 100x150", bounds.Dx(), bounds.Dy())
	}
}

func TestExtractCover_NoCover(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	opts.HasCover = false
	epubPath := createTestEPUB(t, dir, "no-cover.epub", opts)
	destPath := filepath.Join(dir, "cover.jpg")

	err := ExtractCover(epubPath, destPath, 85)
	if err == nil {
		t.Error("expected error for EPUB without cover")
	}
}

// --- CountWordsAndPages tests ---

func TestCountWordsAndPages(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	opts.Content = "<p>One two three four five six seven eight nine ten.</p>"
	epubPath := createTestEPUB(t, dir, "wordcount.epub", opts)

	words, pages, err := CountWordsAndPages(epubPath, 250)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	// 10 words in body + 2 from <title>Chapter 1</title> in head = 12
	if words != 12 {
		t.Errorf("words = %d, want 12", words)
	}
	if pages != 1 {
		t.Errorf("pages = %d, want 1", pages)
	}
}

func TestCountWordsAndPages_LargeContent(t *testing.T) {
	dir := t.TempDir()
	opts := defaultOpts()
	// Generate exactly 500 words
	words := make([]string, 0, 500)
	for range 500 {
		words = append(words, "word")
	}
	opts.Content = "<p>" + strings.Join(words, " ") + "</p>"
	epubPath := createTestEPUB(t, dir, "large.epub", opts)

	w, p, err := CountWordsAndPages(epubPath, 250)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	// 500 words in body + 2 from <title>Chapter 1</title> = 502
	if w != 502 {
		t.Errorf("words = %d, want 502", w)
	}
	// 502/250 = 2.008, rounds up to 3
	if p != 3 {
		t.Errorf("pages = %d, want 3", p)
	}
}

// --- stripXMLTags / countWords tests ---

func TestStripXMLTags(t *testing.T) {
	input := "<html><body><p>Hello <b>world</b>!</p></body></html>"
	got := stripXMLTags(input)
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Errorf("stripped = %q", got)
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello world", 2},
		{"  multiple   spaces  ", 2},
		{"", 0},
		{"one", 1},
		{"word1 word2 word3", 3},
	}
	for _, tt := range tests {
		got := countWords(tt.input)
		if got != tt.want {
			t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- normalizeForSort tests ---

func TestNormalizeForSort(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"The Great Gatsby", "great gatsby"},
		{"A Tale of Two Cities", "tale of two cities"},
		{"An Introduction", "introduction"},
		{"Dune", "dune"},
		{"  The  Spaces  ", "spaces"},
	}
	for _, tt := range tests {
		got := normalizeForSort(tt.input)
		if got != tt.want {
			t.Errorf("normalizeForSort(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
