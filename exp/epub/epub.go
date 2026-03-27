// Package epub provides EPUB metadata parsing, cover extraction, and word counting.
//
// It parses OPF metadata directly rather than relying on third-party EPUB libraries
// that panic on valid EPUBs with missing optional elements.
// Supports EPUB 2, EPUB 3, and Calibre custom metadata.
package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Metadata holds the parsed metadata extracted from an EPUB file.
type Metadata struct {
	Title       string
	SortTitle   string
	Authors     []Author
	ISBN        string
	Publisher   string
	Subjects    []string
	Description string
	Language    string
	Series      string
	SeriesIndex float64
	Edition     string
	PublishDate string
}

// Author represents a creator or contributor extracted from EPUB metadata.
type Author struct {
	Name     string
	SortName string
	Role     string
}

// ErrNoOPF is returned when the EPUB container.xml does not reference an OPF file.
var ErrNoOPF = fmt.Errorf("no OPF path found in container.xml")

// dcPackage is used for parsing the full OPF metadata via Dublin Core namespaces.
type dcPackage struct {
	XMLName  xml.Name   `xml:"package"`
	Metadata dcMetadata `xml:"metadata"`
}

type dcMetadata struct {
	Titles       []dcText    `xml:"title"`
	Creators     []dcCreator `xml:"creator"`
	Contributors []dcCreator `xml:"contributor"`
	Identifiers  []dcID      `xml:"identifier"`
	Publishers   []dcText    `xml:"publisher"`
	Subjects     []dcText    `xml:"subject"`
	Descriptions []dcText    `xml:"description"`
	Languages    []dcText    `xml:"language"`
	Dates        []dcText    `xml:"date"`
	Meta         []opfMeta   `xml:"meta"`
}

type dcText struct {
	Text string `xml:",chardata"`
	Lang string `xml:"lang,attr"`
}

type dcCreator struct {
	Text   string `xml:",chardata"`
	Role   string `xml:"role,attr"`
	FileAs string `xml:"file-as,attr"`
}

type dcID struct {
	Text   string `xml:",chardata"`
	ID     string `xml:"id,attr"`
	Scheme string `xml:"scheme,attr"`
}

// Parse reads an EPUB file and returns its metadata.
func Parse(path string) (*Metadata, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is caller-controlled, not user input
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	containerPath := findContainerOPFPath(zr)
	if containerPath == "" {
		return nil, ErrNoOPF
	}

	pkg, err := parseDCPackage(zr, containerPath)
	if err != nil {
		return nil, err
	}

	meta := &Metadata{}

	// Title
	if len(pkg.Metadata.Titles) > 0 {
		meta.Title = strings.TrimSpace(pkg.Metadata.Titles[0].Text)
		meta.SortTitle = normalizeForSort(meta.Title)
	}

	// Authors
	for _, c := range pkg.Metadata.Creators {
		name := strings.TrimSpace(c.Text)
		if name == "" {
			continue
		}
		role := c.Role
		if role == "" {
			role = "author"
		}
		sortName := c.FileAs
		if sortName == "" {
			sortName = normalizeForSort(name)
		}
		meta.Authors = append(meta.Authors, Author{
			Name:     name,
			SortName: sortName,
			Role:     role,
		})
	}

	// Contributors (skip tool-generated entries like Calibre)
	for _, c := range pkg.Metadata.Contributors {
		name := strings.TrimSpace(c.Text)
		if name == "" {
			continue
		}
		if isToolContributor(name, c.Role) {
			continue
		}
		role := c.Role
		if role == "" {
			role = "contributor"
		}
		sortName := c.FileAs
		if sortName == "" {
			sortName = normalizeForSort(name)
		}
		meta.Authors = append(meta.Authors, Author{
			Name:     name,
			SortName: sortName,
			Role:     role,
		})
	}

	// ISBN
	for _, id := range pkg.Metadata.Identifiers {
		if strings.EqualFold(id.Scheme, "ISBN") {
			meta.ISBN = strings.TrimSpace(id.Text)
			break
		}
	}

	// Publisher
	if len(pkg.Metadata.Publishers) > 0 {
		meta.Publisher = strings.TrimSpace(pkg.Metadata.Publishers[0].Text)
	}

	// Subjects
	for _, s := range pkg.Metadata.Subjects {
		text := strings.TrimSpace(s.Text)
		if text != "" {
			meta.Subjects = append(meta.Subjects, text)
		}
	}

	// Description
	if len(pkg.Metadata.Descriptions) > 0 {
		meta.Description = strings.TrimSpace(pkg.Metadata.Descriptions[0].Text)
	}

	// Language
	if len(pkg.Metadata.Languages) > 0 {
		meta.Language = strings.TrimSpace(pkg.Metadata.Languages[0].Text)
	}

	// Dates
	if len(pkg.Metadata.Dates) > 0 {
		meta.PublishDate = strings.TrimSpace(pkg.Metadata.Dates[0].Text)
	}

	// Series from Calibre metadata or EPUB 3 collection
	meta.Series, meta.SeriesIndex = extractSeriesFromMeta(pkg.Metadata.Meta)

	return meta, nil
}

func parseDCPackage(zr *zip.Reader, opfPath string) (*dcPackage, error) {
	var opfFile *zip.File
	for _, f := range zr.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}
	if opfFile == nil {
		return nil, fmt.Errorf("OPF file not found: %s", opfPath)
	}
	rc, err := opfFile.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	var pkg dcPackage
	if err := xmlDecode(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func extractSeriesFromMeta(metas []opfMeta) (string, float64) {
	var series string
	var seriesIndex float64
	for _, m := range metas {
		switch {
		case m.Name == "calibre:series":
			series = m.Content
		case m.Name == "calibre:series_index":
			if v, err := strconv.ParseFloat(m.Content, 64); err == nil {
				seriesIndex = v
			}
		case m.Property == "belongs-to-collection":
			if series == "" {
				series = strings.TrimSpace(m.Text)
			}
		case m.Property == "group-position":
			if seriesIndex == 0 {
				if v, err := strconv.ParseFloat(strings.TrimSpace(m.Text), 64); err == nil {
					seriesIndex = v
				}
			}
		}
	}
	return series, seriesIndex
}

// isToolContributor returns true if the contributor is a tool-generated entry
// (e.g., Calibre stamps itself as a bkp contributor with its version and URL).
func isToolContributor(name, role string) bool {
	if strings.EqualFold(role, "bkp") {
		return true
	}
	lower := strings.ToLower(name)
	return strings.Contains(lower, "calibre") ||
		strings.Contains(lower, "calibre-ebook.com")
}
