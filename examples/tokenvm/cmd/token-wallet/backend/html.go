// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package backend

import (
	"io"

	"golang.org/x/net/html"
)

// Inspiration: https://gist.github.com/inotnako/c4a82f6723f6ccea5d83c5d3689373dd
type HTMLMeta struct {
	URL  string
	Host string

	SiteName    string
	Title       string
	Description string
	Image       string
}

func ParseHTML(url string, host string, resp io.Reader) *HTMLMeta {
	z := html.NewTokenizer(resp)

	titleFound := false

	hm := new(HTMLMeta)
	hm.URL = url
	hm.Host = host

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return hm
		case html.StartTagToken, html.SelfClosingTagToken:
			// TODO: to handle reddit, we need to wait for page to load before parsing
			// TODO: to handle twitter, need to fix many redirect issue
			t := z.Token()
			if t.Data == `body` {
				return hm
			}
			if t.Data == "title" {
				titleFound = true
			}
			if t.Data == "meta" {
				desc, ok := extractMetaProperty(t, "description")
				if ok {
					hm.Description = desc
				}

				ogTitle, ok := extractMetaProperty(t, "og:title")
				if ok {
					hm.Title = ogTitle
				}

				ogDesc, ok := extractMetaProperty(t, "og:description")
				if ok {
					hm.Description = ogDesc
				}

				ogImage, ok := extractMetaProperty(t, "og:image")
				if ok {
					hm.Image = ogImage
				}

				ogSiteName, ok := extractMetaProperty(t, "og:site_name")
				if ok {
					hm.SiteName = ogSiteName
				}
			}
		case html.TextToken:
			t := z.Token()
			if titleFound {
				if len(hm.Title) == 0 {
					hm.Title = t.Data
				}
				titleFound = false
			}
		}
	}
}

func extractMetaProperty(t html.Token, prop string) (content string, ok bool) {
	for _, attr := range t.Attr {
		if attr.Key == "property" && attr.Val == prop {
			ok = true
		}

		if attr.Key == "content" {
			content = attr.Val
		}
	}

	return
}
