package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

const maxVisible = 8

type fileSuggest struct {
	active   bool
	query    string
	items    []string // all filtered results (no cap)
	selected int
	offset   int      // scroll offset for visible window
	allFiles []string // cached absolute paths
	baseDir  string
	atPos    int // position of '@' in textarea value
}

func newFileSuggest(baseDir string) fileSuggest {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		abs = baseDir
	}
	return fileSuggest{
		baseDir: abs,
	}
}

func (fs *fileSuggest) activate(atPos int) {
	fs.active = true
	fs.query = ""
	fs.selected = 0
	fs.offset = 0
	fs.atPos = atPos
	if fs.allFiles == nil {
		fs.allFiles = walkFiles(fs.baseDir)
	}
	fs.items = fs.filter("")
}

func (fs *fileSuggest) deactivate() {
	fs.active = false
	fs.query = ""
	fs.items = nil
	fs.selected = 0
	fs.offset = 0
}

func (fs *fileSuggest) appendQuery(ch rune) {
	fs.query += string(ch)
	fs.items = fs.filter(fs.query)
	fs.selected = 0
	fs.offset = 0
}

func (fs *fileSuggest) backspaceQuery() {
	if len(fs.query) == 0 {
		fs.deactivate()
		return
	}
	fs.query = fs.query[:len(fs.query)-1]
	fs.items = fs.filter(fs.query)
	fs.selected = 0
	fs.offset = 0
}

func (fs *fileSuggest) moveUp() {
	if fs.selected > 0 {
		fs.selected--
		if fs.selected < fs.offset {
			fs.offset = fs.selected
		}
	}
}

func (fs *fileSuggest) moveDown() {
	if fs.selected < len(fs.items)-1 {
		fs.selected++
		if fs.selected >= fs.offset+maxVisible {
			fs.offset = fs.selected - maxVisible + 1
		}
	}
}

func (fs *fileSuggest) confirm() string {
	if len(fs.items) == 0 {
		return ""
	}
	return fs.items[fs.selected]
}

func (fs *fileSuggest) filter(query string) []string {
	q := strings.ToLower(query)
	var matches []string
	for _, f := range fs.allFiles {
		if q == "" || strings.Contains(strings.ToLower(filepath.Base(f)), q) || strings.Contains(strings.ToLower(f), q) {
			matches = append(matches, f)
		}
	}
	return matches
}

func (fs fileSuggest) View(width int) string {
	if !fs.active || len(fs.items) == 0 {
		return ""
	}

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorBright).
		Background(lipgloss.Color("#3E4451"))
	normalStyle := lipgloss.NewStyle().
		Foreground(colorText)
	hdrStyle := lipgloss.NewStyle().
		Foreground(colorFaint).
		Italic(true)
	countStyle := lipgloss.NewStyle().
		Foreground(colorFaint)

	var lines []string

	queryDisplay := fs.query
	if queryDisplay == "" {
		queryDisplay = "..."
	}
	lines = append(lines, hdrStyle.Render("  @ "+queryDisplay))

	end := fs.offset + maxVisible
	if end > len(fs.items) {
		end = len(fs.items)
	}
	visible := fs.items[fs.offset:end]

	if fs.offset > 0 {
		lines = append(lines, countStyle.Render("  ↑ more"))
	}

	for i, item := range visible {
		idx := fs.offset + i
		display := filepath.Base(item)
		if idx == fs.selected {
			lines = append(lines, selectedStyle.Width(width-6).Render("► "+display+"  "+countStyle.Render(item)))
		} else {
			lines = append(lines, normalStyle.Width(width-6).Render("  "+display+"  "+countStyle.Render(item)))
		}
	}

	if end < len(fs.items) {
		lines = append(lines, countStyle.Render("  ↓ more"))
	}

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorTool).
		Padding(0, 1).
		Width(width - 4).
		Render(strings.Join(lines, "\n"))
}

func walkFiles(baseDir string) []string {
	var files []string
	ignored := map[string]bool{
		".git": true, ".idea": true, "node_modules": true,
		"vendor": true, "__pycache__": true, ".vscode": true,
	}

	_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if ignored[name] || (len(name) > 1 && name[0] == '.') {
				return filepath.SkipDir
			}
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(abs))
		return nil
	})

	sort.Strings(files)
	return files
}
