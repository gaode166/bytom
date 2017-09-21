// Package static provides a handler for serving static assets from an in-memory
// map.
package static

import (
	"net/http"
	"strings"
	"time"
)

// use start time as a conservative bound for last-modified
var lastMod = time.Now()

type Handler struct {
	Assets map[string]string

	// Index is the name of an entry in Assets that should be used if the request
	// path is empty (equivalent to requesting "/"). This is analogous to index
	// documents commonly used in webservers. If Index is empty, it will be
	// ignored.
	Index string

	// Default is the name of an entry in Assets that should be used if the
	// the requested path does not exist in Assets. This is useful for
	// delivering a common document (usually a frontend application script) that
	// handles URL-based state on the client side. If Default is empty, it will be
	// ignored.
	Default string
}

func (h Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	output, ok := h.Assets[r.URL.Path]
	if !ok && r.URL.Path == "" && h.Index != "" {
		output = h.Assets[h.Index]
	} else if !ok && h.Default != "" {
		output = h.Assets[h.Default]
	} else if !ok {
		http.NotFound(rw, r)
		return
	}

	// Some autogenerated documentation uses frames, e.g. Javadoc
	rw.Header().Set("X-Frame-Options", "SAMEORIGIN")

	http.ServeContent(rw, r, r.URL.Path, lastMod, strings.NewReader(output))
}