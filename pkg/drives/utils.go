package drives

import (
	"compress/gzip"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func AddVersionSuffix(source string, fs afero.Fs, isDir bool) string {
	counter := 1
	dir, name := path.Split(source)
	ext := ""
	base := name
	if !isDir {
		ext = filepath.Ext(name)
		base = strings.TrimSuffix(name, ext)
	}

	for {
		if fs == nil {
			if _, err := os.Stat(source); err != nil {
				break
			}
		} else {
			if _, err := fs.Stat(source); err != nil {
				break
			}
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func SuitableResponseReader(resp *http.Response) io.ReadCloser {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return nil
		}
		return &autoCloseReader{
			Reader: gzipReader,
			closer: resp.Body,
		}
	}
	return resp.Body
}

type autoCloseReader struct {
	io.Reader
	closer io.Closer
}

func (a *autoCloseReader) Close() error {
	return a.closer.Close()
}

func RemoveAdditionalHeaders(header *http.Header) {
	header.Del("Traceparent")
	header.Del("Tracestate")
	return
}
