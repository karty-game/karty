package serve

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Release installation fetches named startup assets with redirects disabled.
// FileServer redirects explicit index.html URLs; serve those files directly
// while retaining ordinary file-server behavior for directory and asset URLs.
func previewFiles(directory string) http.Handler {
	files := http.Dir(directory)
	fallback := http.FileServer(files)

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		name := request.URL.Path
		if !strings.HasSuffix(name, "/index.html") || path.Clean(name) != name {
			fallback.ServeHTTP(writer, request)

			return
		}

		file, err := files.Open(name)
		if err != nil {
			previewFileError(writer, request, err)

			return
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			previewFileError(writer, request, err)

			return
		}

		if info.IsDir() {
			fallback.ServeHTTP(writer, request)

			return
		}

		http.ServeContent(writer, request, info.Name(), info.ModTime(), file)
	})
}

func previewFileError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		http.NotFound(writer, request)
	case errors.Is(err, fs.ErrPermission):
		http.Error(writer, "Forbidden", http.StatusForbidden)
	default:
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
	}
}
