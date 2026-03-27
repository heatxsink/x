package epub

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path"
	"strings"
)

// opfPackage is a minimal OPF parser for cover image references,
// manifest items, spine order, and series metadata.
type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
}

type opfMetadata struct {
	Meta []opfMeta `xml:"meta"`
}

type opfMeta struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Property string `xml:"property,attr"`
	Text     string `xml:",chardata"`
}

type opfManifest struct {
	Items []opfItem `xml:"item"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type opfSpine struct {
	ItemRefs []opfItemRef `xml:"itemref"`
}

type opfItemRef struct {
	IDRef string `xml:"idref,attr"`
}

func parseOPF(zr *zip.Reader, opfPath string) (*opfPackage, string, error) {
	var opfFile *zip.File
	for _, f := range zr.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}
	if opfFile == nil {
		return nil, "", nil
	}
	rc, err := opfFile.Open()
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", err
	}
	var pkg opfPackage
	if err := xmlDecode(data, &pkg); err != nil {
		return nil, "", err
	}
	opfDir := path.Dir(opfPath)
	if opfDir == "." {
		opfDir = ""
	}
	return &pkg, opfDir, nil
}

// findCoverImagePath locates the cover image file path within the EPUB.
// Checks EPUB 3 properties="cover-image" first, then EPUB 2 <meta name="cover">.
func findCoverImagePath(zr *zip.Reader, opfPath string) string {
	pkg, opfDir, err := parseOPF(zr, opfPath)
	if err != nil || pkg == nil {
		return ""
	}

	// EPUB 3: manifest item with properties="cover-image"
	for _, item := range pkg.Manifest.Items {
		if strings.Contains(item.Properties, "cover-image") {
			return joinPath(opfDir, item.Href)
		}
	}

	// EPUB 2: <meta name="cover" content="cover-id"/> -> lookup manifest by id
	var coverID string
	for _, m := range pkg.Metadata.Meta {
		if m.Name == "cover" {
			coverID = m.Content
			break
		}
	}
	if coverID == "" {
		return ""
	}
	for _, item := range pkg.Manifest.Items {
		if item.ID == coverID && isImageType(item.MediaType) {
			return joinPath(opfDir, item.Href)
		}
	}
	return ""
}

// spineContentPaths returns the file paths for all XHTML content documents in spine order.
func spineContentPaths(zr *zip.Reader, opfPath string) []string {
	pkg, opfDir, err := parseOPF(zr, opfPath)
	if err != nil || pkg == nil {
		return nil
	}
	idToHref := make(map[string]string, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		idToHref[item.ID] = item.Href
	}
	var paths []string
	for _, ref := range pkg.Spine.ItemRefs {
		if href, ok := idToHref[ref.IDRef]; ok {
			paths = append(paths, joinPath(opfDir, href))
		}
	}
	return paths
}

func joinPath(dir, href string) string {
	if dir == "" {
		return href
	}
	return dir + "/" + href
}

func isImageType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "image/")
}
