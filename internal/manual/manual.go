package manual

import (
	"embed"
	"errors"
	"io/fs"
	"sort"
	"strings"
)

//go:embed chapters/*.md
var chapters embed.FS

type Hit struct {
	File    string `json:"file"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

func Files() []string {
	items, _ := fs.Glob(chapters, "chapters/*.md")
	out := make([]string, 0, len(items))
	for _, s := range items {
		out = append(out, strings.TrimPrefix(s, "chapters/"))
	}
	sort.Strings(out)
	return out
}
func Chapter(name string) (string, error) {
	if strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".md") {
		return "", errors.New("invalid chapter name")
	}
	data, err := chapters.ReadFile("chapters/" + name)
	return string(data), err
}
func Search(query string, limit int) []Hit {
	query = strings.TrimSpace(strings.ToLower(query))
	if len(query) < 2 {
		return []Hit{}
	}
	if limit < 1 || limit > 20 {
		limit = 12
	}
	hits := []Hit{}
	for _, file := range Files() {
		data, err := Chapter(file)
		if err != nil {
			continue
		}
		lines := strings.Split(data, "\n")
		heading := file
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				heading = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
			if strings.Contains(strings.ToLower(line), query) {
				text := line
				if len(text) > 250 {
					text = text[:250] + "…"
				}
				hits = append(hits, Hit{File: file, Title: heading, Snippet: text})
				if len(hits) >= limit {
					return hits
				}
			}
		}
	}
	return hits
}
