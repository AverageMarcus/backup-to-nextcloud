package main

import (
	"encoding/xml"
	"fmt"
	"io"
)

// Namespace prefixes used in the DAV XML
const (
	NSDAV    = "DAV:"
	NSSabre  = "http://sabredav.org/ns"
	NSOwn    = "http://owncloud.org/ns"
	NSNextcl = "http://nextcloud.org/ns"
)

// Multistatus represents the root <d:multistatus> element
type Multistatus struct {
	Responses []Response `xml:"DAV:response"`
}

// Response represents a <d:response> element containing path and status info
type Response struct {
	Href      string     `xml:"DAV:href"`
	Propstats []Propstat `xml:"DAV:propstat"`
}

// Propstat contains a set of properties and their HTTP status
type Propstat struct {
	Prop   Prop   `xml:"DAV:prop"`
	Status string `xml:"DAV:status"`
}

// Prop holds the various DAV properties (getlastmodified, getetag, etc.)
type Prop struct {
	GetLastModified     string `xml:"DAV:getlastmodified"`
	GetContentLength    *int64 `xml:"DAV:getcontentlength"`
	GetContentType      string `xml:"DAV:getcontenttype"`
	GetETag             string `xml:"DAV:getetag"`
	QuotaUsedBytes      *int64 `xml:"DAV:quota-used-bytes"`
	QuotaAvailableBytes *int64 `xml:"DAV:quota-available-bytes"`
	// ResourceType is special - it's a single element that may contain
	// a <d:collection/> child (indicating a directory) or be empty (file)
	ResourceType ResourceType `xml:"DAV:resourcetype"`
}

// ResourceType determines if a resource is a collection (directory) or a file
type ResourceType struct {
	Collection bool `xml:"DAV:collection"`
}

// UnmarshalXML implements custom XML unmarshaling for Prop
// to handle properties that may not always be present
func (p *Prop) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "getlastmodified":
				var content string
				if err := d.DecodeElement(&content, &elem); err != nil {
					return err
				}
				p.GetLastModified = content
			case "getcontentlength":
				var content string
				if err := d.DecodeElement(&content, &elem); err != nil {
					return err
				}
				if val, err := parseInt64(content); err == nil {
					p.GetContentLength = &val
				}
			case "getcontenttype":
				var content string
				if err := d.DecodeElement(&content, &elem); err != nil {
					return err
				}
				p.GetContentType = content
			case "getetag":
				var content string
				if err := d.DecodeElement(&content, &elem); err != nil {
					return err
				}
				p.GetETag = content
			case "quota-used-bytes":
				var content string
				if err := d.DecodeElement(&content, &elem); err != nil {
					return err
				}
				if val, err := parseInt64(content); err == nil {
					p.QuotaUsedBytes = &val
				}
			case "quota-available-bytes":
				var content string
				if err := d.DecodeElement(&content, &elem); err != nil {
					return err
				}
				if val, err := parseInt64(content); err == nil {
					p.QuotaAvailableBytes = &val
				}
			case "resourcetype":
				// For resourcetype, we need to check if it contains <collection/>
				// rather than just counting children
				for {
					innerTok, err := d.Token()
					if err != nil {
						return err
					}
					if innerTok, ok := innerTok.(xml.EndElement); ok && innerTok.Name.Local == "resourcetype" {
						break
					}
					if innerTok, ok := innerTok.(xml.StartElement); ok && innerTok.Name.Local == "collection" {
						p.ResourceType.Collection = true
						// Skip past the collection element
						for {
							innerTok, err := d.Token()
							if err != nil {
								return err
							}
							if end, ok := innerTok.(xml.EndElement); ok && end.Name.Local == "collection" {
								break
							}
						}
					}
				}
			}
		case xml.EndElement:
			if elem.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// UnmarshalXML handles multistatus response parsing
func (m *Multistatus) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	m.Responses = []Response{}

	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "response" {
			var resp Response
			if err := d.DecodeElement(&resp, &start); err != nil {
				return err
			}
			m.Responses = append(m.Responses, resp)
		}

		if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "multistatus" {
			return nil
		}
	}
}

// UnmarshalXML handles response parsing
func (r *Response) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "href":
				if err := d.DecodeElement(&r.Href, &elem); err != nil {
					return err
				}
			case "propstat":
				var ps Propstat
				if err := d.DecodeElement(&ps, &elem); err != nil {
					return err
				}
				r.Propstats = append(r.Propstats, ps)
			}
		case xml.EndElement:
			if elem.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// UnmarshalXML handles propstat parsing
func (p *Propstat) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "prop":
				if err := d.DecodeElement(&p.Prop, &elem); err != nil {
					return err
				}
			case "status":
				var content string
				if err := d.DecodeElement(&content, &elem); err != nil {
					return err
				}
				p.Status = content
			}
		case xml.EndElement:
			if elem.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// RemoveQuotes removes surrounding double quotes from a string (for etags like `"abc123"`)
func RemoveQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseInt64 converts a string to int64, returning 0 and error on failure
func parseInt64(s string) (int64, error) {
	var val int64
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

// IsCollection returns true if the resource is a directory
func (r *Response) IsCollection() bool {
	for _, ps := range r.Propstats {
		if ps.Prop.ResourceType.Collection {
			return true
		}
	}
	return false
}

func (r *Response) GetLastModified() string {
	for _, ps := range r.Propstats {
		if ps.Prop.GetLastModified != "" {
			return ps.Prop.GetLastModified
		}
	}
	return ""
}
