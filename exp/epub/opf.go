package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html/charset"
)

// opfPackage is a minimal OPF parser for cover image references,
// manifest items, spine order, and series metadata.
type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
	Guide    opfGuide    `xml:"guide"`
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

// opfGuide is the EPUB 2 <guide> element. Its <reference type="cover">
// is the third place a cover can be declared, after EPUB 3 manifest
// properties and the EPUB 2 <meta name="cover"> entry.
type opfGuide struct {
	References []opfReference `xml:"reference"`
}

type opfReference struct {
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
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
//
// Resolution order, from most to least authoritative:
//  1. EPUB 3: manifest item with properties containing "cover-image".
//  2. EPUB 2: <meta name="cover" content="ID"/> pointing at a manifest item.
//  3. EPUB 2: <guide><reference type="cover" href=.../></guide>.
//  4. Heuristic: a manifest image whose id or file name is "cover".
//  5. Heuristic: a manifest image whose id or file name contains "cover".
//
// Any step that lands on an XHTML document (a "cover page") is followed one
// hop further to the first <img> or SVG <image> inside it. Every href is
// resolved as a URI relative to the OPF: percent-decoded, fragment-stripped,
// and cleaned of ./ and ../ segments so it matches a zip entry name.
func findCoverImagePath(zr *zip.Reader, opfPath string) string {
	pkg, opfDir, err := parseOPF(zr, opfPath)
	if err != nil || pkg == nil {
		return ""
	}

	// 1. EPUB 3 manifest property.
	for _, item := range pkg.Manifest.Items {
		if hasProperty(item.Properties, "cover-image") {
			return resolveHref(opfDir, item.Href)
		}
	}

	// 2. EPUB 2 <meta name="cover">.
	for _, m := range pkg.Metadata.Meta {
		if m.Name != "cover" || m.Content == "" {
			continue
		}
		if item := manifestItemByID(pkg, m.Content); item != nil {
			if p := coverFromItem(zr, opfDir, item); p != "" {
				return p
			}
		}
		// Some generators put the href, not the id, in content=.
		if p := coverFromHref(zr, opfDir, m.Content); p != "" {
			return p
		}
	}

	// 3. EPUB 2 guide reference.
	for _, ref := range pkg.Guide.References {
		if strings.EqualFold(ref.Type, "cover") {
			if p := coverFromHref(zr, opfDir, ref.Href); p != "" {
				return p
			}
		}
	}

	// 4. Exact-name heuristic: id or basename equals "cover".
	for _, item := range pkg.Manifest.Items {
		if !isRasterItem(item) {
			continue
		}
		if strings.EqualFold(item.ID, "cover") || strings.EqualFold(baseName(item.Href), "cover") {
			return resolveHref(opfDir, item.Href)
		}
	}
	// A "cover" XHTML page with no image marker at all.
	for _, item := range pkg.Manifest.Items {
		if !isXHTMLItem(item) {
			continue
		}
		if strings.EqualFold(item.ID, "cover") || strings.EqualFold(baseName(item.Href), "cover") {
			if p := imageFromXHTML(zr, resolveHref(opfDir, item.Href)); p != "" {
				return p
			}
		}
	}

	// 5. Loose heuristic: id or basename contains "cover".
	for _, item := range pkg.Manifest.Items {
		if !isRasterItem(item) {
			continue
		}
		if containsFold(item.ID, "cover") || containsFold(baseName(item.Href), "cover") {
			return resolveHref(opfDir, item.Href)
		}
	}

	return ""
}

// coverFromItem returns the image path a manifest item denotes: itself when it
// is an image, or the first image inside it when it is an XHTML cover page.
func coverFromItem(zr *zip.Reader, opfDir string, item *opfItem) string {
	full := resolveHref(opfDir, item.Href)
	switch {
	case isImageItem(*item):
		return full
	case isXHTMLItem(*item):
		return imageFromXHTML(zr, full)
	}
	return ""
}

// coverFromHref is coverFromItem for a bare href with no manifest entry to
// tell us the media type; it decides by extension and file contents instead.
func coverFromHref(zr *zip.Reader, opfDir, href string) string {
	full := resolveHref(opfDir, href)
	if full == "" {
		return ""
	}
	if hasImageExt(full) {
		return full
	}
	if hasXHTMLExt(full) {
		return imageFromXHTML(zr, full)
	}
	return ""
}

// imageFromXHTML opens an XHTML document inside the archive and returns the
// resolved path of the first <img src> or SVG <image href> it contains.
func imageFromXHTML(zr *zip.Reader, xhtmlPath string) string {
	f := findZipFile(zr, xhtmlPath)
	if f == nil {
		return ""
	}
	rc, err := f.Open()
	if err != nil {
		return ""
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return ""
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	dec.CharsetReader = charset.NewReaderLabel

	dir := path.Dir(xhtmlPath)
	if dir == "." {
		dir = ""
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		var want string
		switch strings.ToLower(se.Name.Local) {
		case "img":
			want = "src"
		case "image":
			want = "href"
		default:
			continue
		}
		for _, a := range se.Attr {
			if strings.EqualFold(a.Name.Local, want) && a.Value != "" {
				return resolveHref(dir, a.Value)
			}
		}
	}
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
			paths = append(paths, resolveHref(opfDir, href))
		}
	}
	return paths
}

// resolveHref turns a manifest href, which is a relative URI, into a zip
// entry name relative to the archive root. It strips any fragment or query,
// percent-decodes ("cover%20art.jpg"), and joins with the OPF directory
// through path.Join so "./" and "../" segments collapse.
func resolveHref(dir, href string) string {
	if i := strings.IndexAny(href, "#?"); i >= 0 {
		href = href[:i]
	}
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	if href == "" {
		return ""
	}
	return path.Join(dir, href)
}

// findZipFile returns the archive entry with the given name. It tries an
// exact match first, then a case-insensitive one for archives produced on
// case-insensitive filesystems whose OPF hrefs disagree with the entry names.
func findZipFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

func manifestItemByID(pkg *opfPackage, id string) *opfItem {
	for i := range pkg.Manifest.Items {
		if pkg.Manifest.Items[i].ID == id {
			return &pkg.Manifest.Items[i]
		}
	}
	return nil
}

// hasProperty reports whether a space-separated OPF properties attribute
// contains the exact token.
func hasProperty(properties, want string) bool {
	for _, p := range strings.Fields(properties) {
		if p == want {
			return true
		}
	}
	return false
}

func isImageType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "image/")
}

// isImageItem accepts an item by media type, or by extension when the
// manifest omits or mislabels the media type.
func isImageItem(item opfItem) bool {
	return isImageType(item.MediaType) || hasImageExt(item.Href)
}

// isRasterItem is isImageItem restricted to formats ExtractCover can decode.
// Used by the heuristic passes so we never guess our way onto an SVG.
func isRasterItem(item opfItem) bool {
	switch strings.ToLower(item.MediaType) {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	}
	return hasRasterExt(item.Href)
}

func isXHTMLItem(item opfItem) bool {
	mt := strings.ToLower(item.MediaType)
	return mt == "application/xhtml+xml" || mt == "text/html" || hasXHTMLExt(item.Href)
}

func hasImageExt(href string) bool {
	return hasRasterExt(href) || strings.EqualFold(extOf(href), ".svg")
}

func hasRasterExt(href string) bool {
	switch strings.ToLower(extOf(href)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

func hasXHTMLExt(href string) bool {
	switch strings.ToLower(extOf(href)) {
	case ".xhtml", ".html", ".htm":
		return true
	}
	return false
}

// extOf is path.Ext on the href with any fragment or query removed.
func extOf(href string) string {
	if i := strings.IndexAny(href, "#?"); i >= 0 {
		href = href[:i]
	}
	return path.Ext(href)
}

// baseName is the file name of an href without its extension.
func baseName(href string) string {
	b := path.Base(resolveHref("", href))
	return strings.TrimSuffix(b, path.Ext(b))
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
