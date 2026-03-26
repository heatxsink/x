package epub

import (
	"bytes"
	"encoding/xml"

	"golang.org/x/net/html/charset"
)

// xmlDecode unmarshals XML data into v, supporting non-UTF-8 encodings
// (e.g., iso-8859-1) that are common in older EPUBs.
func xmlDecode(data []byte, v any) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	return decoder.Decode(v)
}
