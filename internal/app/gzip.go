package app

import (
	"io"
	"net/http"
	"strings"
)

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	contentType := g.Header().Get("Content-Type")
	// Дополнительная проверка здесь, на случай если в одном из middleware хендлеров при масштабировании будет изменяться contentType
	if strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html") {
		return g.Writer.Write(b)
	}
	return g.ResponseWriter.Write(b)
}
