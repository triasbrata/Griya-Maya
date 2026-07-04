package convert

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// epubExtractor reads an EPUB. It resolves reading order from the OPF spine for
// fixed-layout (image-per-page) comic EPUBs, falling back to all embedded
// images in natural order when the spine yields nothing.
type epubExtractor struct{}

type opfPackage struct {
	Manifest struct {
		Items []struct {
			ID        string `xml:"id,attr"`
			Href      string `xml:"href,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Refs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

type container struct {
	Rootfiles struct {
		Rootfile []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfile"`
	} `xml:"rootfiles"`
}

var imgSrcRE = regexp.MustCompile(`(?i)(?:src|xlink:href|href)\s*=\s*["']([^"']+\.(?:jpe?g|png|gif|webp))["']`)

func (epubExtractor) Extract(archive []byte) ([]rawPage, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("%w: open epub: %v", domain.ErrInvalidInput, err)
	}
	files := indexZip(zr)

	pages := extractEPUBSpine(files)
	if len(pages) == 0 {
		pages = extractAllImages(files) // fallback
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("%w: epub has no images", domain.ErrInvalidInput)
	}
	return pages, nil
}

// extractEPUBSpine walks container.xml -> OPF -> spine and pulls one image per
// XHTML page in reading order.
func extractEPUBSpine(files map[string][]byte) []rawPage {
	containerXML, ok := files["META-INF/container.xml"]
	if !ok {
		return nil
	}
	var c container
	if err := xml.Unmarshal(containerXML, &c); err != nil || len(c.Rootfiles.Rootfile) == 0 {
		return nil
	}
	opfPath := c.Rootfiles.Rootfile[0].FullPath
	opfData, ok := files[opfPath]
	if !ok {
		return nil
	}
	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return nil
	}
	opfDir := path.Dir(opfPath)

	idToHref := map[string]string{}
	for _, it := range pkg.Manifest.Items {
		idToHref[it.ID] = it.Href
	}

	var pages []rawPage
	seen := map[string]bool{}
	for _, ref := range pkg.Spine.Refs {
		href, ok := idToHref[ref.IDRef]
		if !ok {
			continue
		}
		docPath := resolve(opfDir, href)
		var imgHref string
		if isImageName(docPath) {
			imgHref = docPath // spine item is the image itself
		} else if doc, ok := files[docPath]; ok {
			m := imgSrcRE.FindSubmatch(doc)
			if m == nil {
				continue
			}
			imgHref = resolve(path.Dir(docPath), string(m[1]))
		} else {
			continue
		}
		if seen[imgHref] {
			continue
		}
		if data, ok := files[imgHref]; ok {
			seen[imgHref] = true
			pages = append(pages, rawPage{name: imgHref, data: data})
		}
	}
	return pages
}

func extractAllImages(files map[string][]byte) []rawPage {
	var pages []rawPage
	for name, data := range files {
		if isImageName(name) {
			pages = append(pages, rawPage{name: name, data: data})
		}
	}
	sortPagesNaturally(pages)
	return pages
}

func indexZip(zr *zip.Reader) map[string][]byte {
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		out[f.Name] = data
	}
	return out
}

// resolve joins an href relative to a base dir, cleaning "./" and "../".
func resolve(base, href string) string {
	href = strings.TrimPrefix(href, "./")
	if base == "" || base == "." {
		return path.Clean(href)
	}
	return path.Clean(base + "/" + href)
}
