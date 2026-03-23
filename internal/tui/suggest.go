package tui

import (
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

const maxVisible = 8

type overlayItem struct {
	Primary   string
	Secondary string
}

type selectionOverlayConfig struct {
	Title     string
	QueryIcon string
	Query     string
	Items     []overlayItem
	Selected  int
	Offset    int
	Width     int
	Accent    color.Color
}

type fileSuggest struct {
	active   bool
	dirsOnly bool
	query    string
	items    []string // all filtered results (no cap)
	selected int
	offset   int      // scroll offset for visible window
	allFiles []string // cached absolute paths
	baseDir  string
	atPos    int // position of '@' in textarea value
}

func newFileSuggest(baseDir string) fileSuggest {
	return newPathSuggest(baseDir, false)
}

func newDirSuggest(baseDir string) fileSuggest {
	return newPathSuggest(baseDir, true)
}

func newPathSuggest(baseDir string, dirsOnly bool) fileSuggest {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		abs = baseDir
	}
	return fileSuggest{
		baseDir:  abs,
		dirsOnly: dirsOnly,
	}
}

func (fs *fileSuggest) activate(atPos int) {
	fs.active = true
	fs.query = ""
	fs.selected = 0
	fs.offset = 0
	fs.atPos = atPos
	if fs.allFiles == nil {
		if fs.dirsOnly {
			fs.allFiles = walkDirs(fs.baseDir)
		} else {
			fs.allFiles = walkFiles(fs.baseDir)
		}
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

	items := make([]overlayItem, 0, len(fs.items))
	for _, item := range fs.items {
		items = append(items, overlayItem{
			Primary:   filepath.Base(item),
			Secondary: item,
		})
	}

	title := "Files"
	queryIcon := "@"
	if fs.dirsOnly {
		title = "Directories"
		queryIcon = "/"
	}

	return renderSelectionOverlay(selectionOverlayConfig{
		Title:     title,
		QueryIcon: queryIcon,
		Query:     fs.query,
		Items:     items,
		Selected:  fs.selected,
		Offset:    fs.offset,
		Width:     width,
		Accent:    colorTool,
	})
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

func walkDirs(baseDir string) []string {
	var dirs []string
	ignored := map[string]bool{
		".git": true, ".idea": true, "node_modules": true,
		"vendor": true, "__pycache__": true, ".vscode": true,
	}

	_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != baseDir && (ignored[name] || (len(name) > 1 && name[0] == '.')) {
			return filepath.SkipDir
		}
		if path == baseDir {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil
		}
		dirs = append(dirs, filepath.ToSlash(abs))
		return nil
	})

	sort.Strings(dirs)
	return dirs
}

func renderSelectionOverlay(cfg selectionOverlayConfig) string {
	if len(cfg.Items) == 0 {
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

	queryDisplay := cfg.Query
	if strings.TrimSpace(queryDisplay) == "" {
		queryDisplay = "..."
	}

	lines := []string{
		hdrStyle.Render("  " + cfg.QueryIcon + " " + queryDisplay),
	}

	end := cfg.Offset + maxVisible
	if end > len(cfg.Items) {
		end = len(cfg.Items)
	}
	visible := cfg.Items[cfg.Offset:end]

	if cfg.Offset > 0 {
		lines = append(lines, countStyle.Render("  ↑ more"))
	}

	for i, item := range visible {
		idx := cfg.Offset + i
		line := "  " + item.Primary
		if item.Secondary != "" {
			line += "  " + countStyle.Render(item.Secondary)
		}
		if idx == cfg.Selected {
			lines = append(lines, selectedStyle.Width(cfg.Width-6).Render("► "+item.Primary+"  "+countStyle.Render(item.Secondary)))
		} else {
			lines = append(lines, normalStyle.Width(cfg.Width-6).Render(line))
		}
	}

	if end < len(cfg.Items) {
		lines = append(lines, countStyle.Render("  ↓ more"))
	}

	blockStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(cfg.Accent).
		Padding(0, 1).
		Width(cfg.Width - 4)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(cfg.Accent)

	block := blockStyle.Render(strings.Join(lines, "\n"))
	block = injectBorderTitle(block, titleStyle.Render(cfg.Title), "")
	return block
}
