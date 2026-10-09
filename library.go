package main

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Recording struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Created int64  `json:"created"` // unix ms
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	URL     string `json:"url"`
	Thumb   string `json:"thumb"`
}

func isRecordingName(n string) bool {
	if !strings.HasPrefix(n, "gifkite-") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(n))
	return ext == ".gif" || ext == ".webp" || ext == ".mp4"
}

// ListRecordings returns Gifkite's recordings in the output folder, newest first.
func (s *GifService) ListRecordings() []Recording {
	dir := s.GetState().Settings.OutputDir
	entries, _ := os.ReadDir(dir)
	var out []Recording
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !isRecordingName(n) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		r := Recording{
			Name:    n,
			Size:    info.Size(),
			Created: info.ModTime().UnixMilli(),
			URL:     "/media/" + url.PathEscape(n),
			Thumb:   "/thumb/" + url.PathEscape(n) + "?v=" + info.ModTime().Format("150405.000"),
		}
		if f, err := os.Open(filepath.Join(dir, n)); err == nil {
			if cfg, _, err := image.DecodeConfig(f); err == nil {
				r.Width, r.Height = cfg.Width, cfg.Height
			} else {
				// Fallback to gif
				f.Seek(0, 0)
				if gcfg, err := gif.DecodeConfig(f); err == nil {
					r.Width, r.Height = gcfg.Width, gcfg.Height
				}
			}
			f.Close()
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created > out[j].Created })
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

// recordingPath resolves a name from the frontend to a file in the output
// folder, refusing anything that isn't a plain gifkite filename.
func (s *GifService) recordingPath(name string) (string, error) {
	if name != filepath.Base(name) || !isRecordingName(name) {
		return "", errors.New("not a gifkite recording")
	}
	return filepath.Join(s.GetState().Settings.OutputDir, name), nil
}

// CopyFile puts the GIF itself on the clipboard (not its path), so it
// pastes as an attachment into Slack, Messages, Finder, etc.
func (s *GifService) CopyFile(name string) error {
	p, err := s.recordingPath(name)
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return copyDarwinFile(p)
	case "windows":
		return exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard -Path '"+strings.ReplaceAll(p, "'", "''")+"'").Run()
	default:
		uri := "file://" + (&url.URL{Path: p}).EscapedPath() + "\n"
		for _, c := range [][]string{{"wl-copy", "--type", "text/uri-list"}, {"xclip", "-selection", "clipboard", "-t", "text/uri-list"}} {
			cmd := exec.Command(c[0], c[1:]...)
			cmd.Stdin = strings.NewReader(uri)
			if cmd.Run() == nil {
				return nil
			}
		}
		return errors.New("install wl-clipboard or xclip to copy files")
	}
}

// StartDrag initiates a native OS file drag from the popover window.
func (s *GifService) StartDrag(name string) error {
	p, err := s.recordingPath(name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.isDragging = true
	s.mu.Unlock()
	startNativeFileDrag(s.popover.NativeWindow(), p)
	return nil
}

func (s *GifService) Reveal(name string) error {
	p, err := s.recordingPath(name)
	if err != nil {
		return err
	}
	s.popover.Hide()
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", p).Start()
	case "windows":
		return exec.Command("explorer", "/select,", p).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(p)).Start()
	}
}

func (s *GifService) Open(name string) error {
	p, err := s.recordingPath(name)
	if err != nil {
		return err
	}
	s.popover.Hide()
	return openPath(p)
}

func (s *GifService) Delete(name string) error {
	p, err := s.recordingPath(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func (s *GifService) OpenFolder() {
	s.popover.Hide()
	openPath(s.GetState().Settings.OutputDir)
}

func openPath(p string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", p).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", p).Start()
	default:
		return exec.Command("xdg-open", p).Start()
	}
}

// ---- asset middleware ----

// middleware serves the recordings, their thumbnails and the frozen
// screenshot to the webviews; everything else falls through to the
// embedded frontend.
func (s *GifService) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/frozen.jpg":
			s.mu.Lock()
			b := s.frozen
			s.mu.Unlock()
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(b)
		case r.URL.Path == "/preview/frame":
			q := r.URL.Query().Get("i")
			idx, err := strconv.Atoi(q)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			b := s.GetPreviewFrame(idx)
			if b == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(b)
		case strings.HasPrefix(r.URL.Path, "/media/"):
			p, err := s.recordingPath(strings.TrimPrefix(r.URL.Path, "/media/"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			ext := strings.ToLower(filepath.Ext(p))
			switch ext {
			case ".webp":
				w.Header().Set("Content-Type", "image/webp")
			case ".mp4":
				w.Header().Set("Content-Type", "video/mp4")
			default:
				w.Header().Set("Content-Type", "image/gif")
			}
			http.ServeFile(w, r, p)
		case strings.HasPrefix(r.URL.Path, "/thumb/"):
			p, err := s.recordingPath(strings.TrimPrefix(r.URL.Path, "/thumb/"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			b, err := s.thumbs.get(p)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(b)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// thumbCache holds a still PNG of each recording's first frame.
type thumbCache struct {
	mu sync.Mutex
	m  map[string]thumbEntry
}

type thumbEntry struct {
	mod time.Time
	png []byte
}

func newThumbCache() *thumbCache { return &thumbCache{m: map[string]thumbEntry{}} }

func (c *thumbCache) put(path string, pngData []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[path] = thumbEntry{mod: time.Now(), png: pngData}
}

func (c *thumbCache) get(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	e, ok := c.m[path]
	c.mu.Unlock()
	if ok && e.mod.Equal(info.ModTime()) {
		return e.png, nil
	}
	first, err := ExtractThumbnailImage(path)
	if err != nil {
		return nil, err
	}
	b := first.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), first, b.Min, draw.Src)
	const maxW = 480 // 2x the displayed width
	if rgba.Bounds().Dx() > maxW {
		h := max(1, rgba.Bounds().Dy()*maxW/rgba.Bounds().Dx())
		rgba = boxScale(rgba, maxW, h)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.m[path] = thumbEntry{mod: info.ModTime(), png: buf.Bytes()}
	c.mu.Unlock()
	return buf.Bytes(), nil
}
