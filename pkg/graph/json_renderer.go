package graph

import (
	"encoding/json"
	"io"
)

// RenderJSON serializes the graph as compact JSON and writes it to w.
// It is a pure renderer with no side effects, no DB access, and no writes
// except to the provided io.Writer.
func RenderJSON(g *Graph, w io.Writer) error {
	if g == nil {
		g = emptyGraph()
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(g)
}
