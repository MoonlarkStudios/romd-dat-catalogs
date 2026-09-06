package publisher

import (
	"encoding/xml"
	"time"
)

const Namespace = "urn:romd:dat-catalog:experimental:1"

type rss struct {
	XMLName   xml.Name `xml:"rss"`
	Version   string   `xml:"version,attr"`
	Namespace string   `xml:"xmlns:romd,attr"`
	Channel   channel  `xml:"channel"`
}
type channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []item `xml:"item"`
}
type guid struct {
	Permanent string `xml:"isPermaLink,attr"`
	Value     string `xml:",chardata"`
}
type enclosure struct {
	URL    string `xml:"url,attr"`
	Length int    `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}
type item struct {
	Title     string    `xml:"title"`
	GUID      guid      `xml:"guid"`
	Date      string    `xml:"pubDate"`
	Link      string    `xml:"link"`
	Enclosure enclosure `xml:"enclosure"`
	CatalogID string    `xml:"romd:catalogId"`
	Hash      string    `xml:"romd:sha256"`
	Sequence  int64     `xml:"romd:sequence"`
}

func feedBytes(events []Event, base string) ([]byte, error) {
	r := rss{Version: "2.0", Namespace: Namespace, Channel: channel{Title: "ROMD DAT catalogs (development)", Link: base, Description: "Catalog document changes; verify signed distribution metadata before applying updates"}}
	for _, e := range events {
		at, err := time.Parse(time.RFC3339Nano, e.PublishedAt)
		if err != nil {
			return nil, err
		}
		u := base + e.Artifact.Path
		r.Channel.Items = append(r.Channel.Items, item{e.Name, guid{"false", e.ID}, at.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"), u, enclosure{u, e.Artifact.Bytes, "application/xml"}, e.CatalogID, e.Artifact.SHA256, e.Sequence})
	}
	b, e := xml.Marshal(r)
	return append([]byte(xml.Header), b...), e
}
