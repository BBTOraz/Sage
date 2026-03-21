package chunking

import (
	"bilge-lib/internal/ingestion/parser"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrNilParsedFile       = errors.New("parsed file is nil")
	ErrUntrustedParsedFile = errors.New("parsed file is untrusted")
	ErrEmptyParsedContent  = errors.New("parsed file content is empty")
)

const (
	minChunkChars      = 800
	maxChunkChars      = 1800
	maxBlockChars      = 1200
	maxChunkParagraphs = 4
)

var (
	multiBreakPattern   = regexp.MustCompile(`\n{3,}`)
	markdownHeadingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	listItemRegex       = regexp.MustCompile(`^(([-*•‣▪◦])|(\d+[\.\)])|([A-Za-zА-Яа-я][\.\)])|(\([0-9A-Za-zА-Яа-я]+\)))\s+\S+`)
	numericHeadingRegex = regexp.MustCompile(`^\d+(\.\d+)*[\).]?\s+\S+`)
	upperHeadingRegex   = regexp.MustCompile(`^[A-Z0-9][A-Z0-9\s\-\:&/]{2,}$`)
	genericHeadingRegex = regexp.MustCompile(`^=+\s*(MAIN CONTENT|CONTENT|DOCUMENT CONTENT)\s*=+$`)
)

type DefaultTransformer struct{}

type blockKind string

const (
	blockParagraph blockKind = "paragraph"
	blockHeading   blockKind = "heading"
	blockList      blockKind = "list"
	blockTable     blockKind = "table"
)

type textBlock struct {
	Text         string
	Kind         blockKind
	HeadingLevel int
}

type headingFrame struct {
	Text  string
	Level int
}

func NewDefaultTransformer() *DefaultTransformer {
	return &DefaultTransformer{}
}

func (t *DefaultTransformer) Transform(ctx context.Context, parsed *parser.ParsedFile) (*ChunkedFile, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if parsed == nil {
		return nil, ErrNilParsedFile
	}
	if parsed.Status == parser.ParseStatusUntrusted {
		return nil, ErrUntrustedParsedFile
	}

	normalized := normalizeContent(parsed.Content)
	if normalized == "" {
		return nil, ErrEmptyParsedContent
	}

	docHash := sha256Hex(normalized)
	docID := DocumentID(sha256Hex(parsed.File.Path + ":" + docHash))
	blocks := splitBlocks(normalized)
	if len(blocks) == 0 {
		return nil, ErrEmptyParsedContent
	}

	title := detectTitle(blocks, parsed.File.Path)
	passport := buildPassport(normalized, blocks)
	source := SourceDocument{
		ID:       docID,
		Path:     parsed.File.Path,
		Name:     filepath.Base(parsed.File.Path),
		FileType: parsed.File.FileType,
		Title:    title,
		DocHash:  docHash,
		Language: passport.Language,
		Passport: passport,
	}

	if stat, err := os.Stat(parsed.File.Path); err == nil {
		source.UpdatedAt = stat.ModTime()
	}

	chunks := buildChunks(blocks, docID)
	if len(chunks) == 0 {
		return nil, ErrEmptyParsedContent
	}

	return &ChunkedFile{
		SourceDocument: source,
		Chunks:         chunks,
	}, nil
}

func normalizeContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = multiBreakPattern.ReplaceAllString(content, "\n\n")
	return strings.TrimSpace(content)
}

func splitBlocks(content string) []textBlock {
	lines := strings.Split(content, "\n")
	blocks := make([]textBlock, 0, len(lines))
	currentKind := blockParagraph
	currentLines := make([]string, 0, 4)

	flush := func() {
		if len(currentLines) == 0 {
			return
		}

		text := renderBlock(currentKind, currentLines)
		if text != "" {
			blocks = append(blocks, textBlock{
				Text: currentKindText(text, currentKind),
				Kind: currentKind,
			})
		}

		currentLines = currentLines[:0]
	}

	for _, rawLine := range lines {
		line := normalizeInlineWhitespace(rawLine)
		if line == "" {
			flush()
			continue
		}
		if isGenericHeading(line) {
			flush()
			continue
		}

		kind, headingLevel := classifyLine(line)
		switch kind {
		case blockHeading:
			flush()
			line = normalizeHeadingText(line)
			if isGenericHeading(line) {
				continue
			}
			blocks = append(blocks, textBlock{
				Text:         line,
				Kind:         blockHeading,
				HeadingLevel: headingLevel,
			})
		case blockList, blockTable:
			if len(currentLines) == 0 {
				currentKind = kind
				currentLines = append(currentLines, line)
				continue
			}
			if currentKind == kind {
				currentLines = append(currentLines, line)
				continue
			}
			flush()
			currentKind = kind
			currentLines = append(currentLines, line)
		default:
			if len(currentLines) == 0 {
				currentKind = blockParagraph
				currentLines = append(currentLines, line)
				continue
			}
			if currentKind == blockParagraph && shouldBreakParagraph(currentLines[len(currentLines)-1], line) {
				flush()
				currentKind = blockParagraph
				currentLines = append(currentLines, line)
				continue
			}
			if currentKind != blockParagraph {
				flush()
				currentKind = blockParagraph
			}
			currentLines = append(currentLines, line)
		}
	}

	flush()

	return blocks
}

func isHeadingLike(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if len(trimmed) > 120 || strings.Contains(trimmed, "\n") {
		return false
	}
	if markdownHeadingRegex.MatchString(trimmed) {
		return true
	}
	if numericHeadingRegex.MatchString(trimmed) {
		return true
	}
	if upperHeadingRegex.MatchString(trimmed) {
		return true
	}
	return looksLikeTitleCaseHeading(trimmed)
}

func detectTitle(blocks []textBlock, path string) string {
	for _, block := range blocks {
		if block.Kind == blockHeading && !isGenericHeading(block.Text) {
			return block.Text
		}
	}
	return filepath.Base(path)
}

func buildChunks(blocks []textBlock, docID DocumentID) []*Chunk {
	chunks := make([]*Chunk, 0, len(blocks))
	sectionStack := make([]headingFrame, 0, 4)
	currentSection := ""
	currentHeading := ""
	currentHeadingLevel := 0
	currentParts := make([]string, 0, maxChunkParagraphs+2)
	currentParagraphs := 0
	offsetCursor := 0

	flush := func() {
		if len(currentParts) == 0 {
			return
		}

		content := renderChunkContent(sectionStack, currentParts)
		start := offsetCursor
		end := start + len(content)
		chunkIndex := len(chunks)

		chunks = append(chunks, &Chunk{
			ID:          ChunkID(chunkID(docID, chunkIndex)),
			DocumentID:  docID,
			ChunkIndex:  chunkIndex,
			Content:     content,
			SectionPath: currentSection,
			Heading:     currentHeading,
			HeadingLevel: currentHeadingLevel,
			StartOffset: start,
			EndOffset:   end,
		})

		offsetCursor = end + 2
		currentParts = currentParts[:0]
		currentParagraphs = 0
	}

	for _, block := range blocks {
		if block.Kind == blockHeading {
			flush()
			sectionStack = updateHeadingStack(sectionStack, block)
			currentSection = sectionPath(sectionStack)
			currentHeading, currentHeadingLevel = currentHeadingInfo(sectionStack)
			continue
		}

		for _, piece := range splitOversizedBlock(block) {
			candidateParts := append(append([]string{}, currentParts...), piece)
			candidateContent := renderChunkContent(sectionStack, candidateParts)

			if len(currentParts) > 0 && (len(candidateContent) > maxChunkChars || currentParagraphs >= maxChunkParagraphs) {
				flush()
				candidateParts = []string{piece}
			}

			currentParts = candidateParts
			currentParagraphs++

			currentContent := renderChunkContent(sectionStack, currentParts)
			if len(currentContent) >= minChunkChars && (currentParagraphs >= maxChunkParagraphs || block.Kind == blockTable) {
				flush()
			}
		}
	}

	flush()

	for i, chunk := range chunks {
		if i > 0 {
			chunk.PrevChunkID = chunks[i-1].ID
		}
		if i < len(chunks)-1 {
			chunk.NextChunkID = chunks[i+1].ID
		}
	}

	return chunks
}

func currentKindText(text string, kind blockKind) string {
	if kind == blockParagraph {
		return mergeParagraphLines(strings.Split(text, "\n"))
	}
	return text
}

func renderBlock(kind blockKind, lines []string) string {
	switch kind {
	case blockList, blockTable:
		return strings.Join(lines, "\n")
	default:
		return mergeParagraphLines(lines)
	}
}

func normalizeInlineWhitespace(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func classifyLine(line string) (blockKind, int) {
	if isHeadingLike(line) {
		return blockHeading, headingLevel(line)
	}
	if isTableLike(line) {
		return blockTable, 0
	}
	if listItemRegex.MatchString(line) {
		return blockList, 0
	}
	return blockParagraph, 0
}

func shouldBreakParagraph(prev, next string) bool {
	prev = strings.TrimSpace(prev)
	next = strings.TrimSpace(next)
	if prev == "" || next == "" {
		return false
	}
	if strings.HasSuffix(prev, "-") {
		return false
	}
	if isHeadingLike(next) || listItemRegex.MatchString(next) || isTableLike(next) {
		return true
	}
	if !endsWithSentenceBoundary(prev) {
		return false
	}
	return looksLikeSentenceStart(next)
}

func mergeParagraphLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	parts := make([]string, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i > 0 && len(parts) > 0 {
			prev := parts[len(parts)-1]
			if strings.HasSuffix(prev, "-") {
				parts[len(parts)-1] = strings.TrimSuffix(prev, "-") + line
				continue
			}
		}
		parts = append(parts, line)
	}

	return strings.Join(parts, " ")
}

func looksLikeTitleCaseHeading(text string) bool {
	if text == "" || strings.ContainsAny(text, ".!?,;") {
		return false
	}

	text = strings.TrimSuffix(text, ":")
	words := strings.Fields(text)
	if len(words) == 0 || len(words) > 12 {
		return false
	}

	contentWords := 0
	strongWords := 0
	for _, word := range words {
		word = strings.Trim(word, "\"'()[]{}")
		if word == "" {
			continue
		}
		if isLowerConnector(word) {
			continue
		}
		contentWords++
		r, _ := utf8DecodeRune(word)
		if unicode.IsUpper(r) || unicode.IsDigit(r) {
			strongWords++
			continue
		}
		if isAllCapsToken(word) {
			strongWords++
		}
	}

	if contentWords == 0 {
		return false
	}
	if contentWords == 1 {
		return strongWords == 1
	}

	return strongWords >= max(2, (contentWords*2+2)/3)
}

func isLowerConnector(word string) bool {
	switch strings.ToLower(word) {
	case "and", "or", "of", "the", "for", "to", "in", "on", "at", "by", "with", "a", "an":
		return true
	default:
		return false
	}
}

func isAllCapsToken(word string) bool {
	hasLetter := false
	for _, r := range word {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

func utf8DecodeRune(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}

func headingLevel(text string) int {
	trimmed := strings.TrimSpace(text)
	if matches := markdownHeadingRegex.FindStringSubmatch(trimmed); len(matches) == 3 {
		return len(matches[1])
	}
	if matches := numericHeadingRegex.FindString(trimmed); matches != "" {
		level := 1 + strings.Count(strings.Fields(matches)[0], ".")
		if level < 1 {
			return 1
		}
		return level
	}
	if upperHeadingRegex.MatchString(trimmed) {
		return 1
	}
	return 2
}

func isTableLike(text string) bool {
	if strings.Count(text, "|") >= 2 || strings.Count(text, "\t") >= 1 {
		return true
	}
	return strings.Count(text, "  ") >= 2
}

func endsWithSentenceBoundary(text string) bool {
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") || strings.HasSuffix(text, ":") || strings.HasSuffix(text, ";")
}

func looksLikeSentenceStart(text string) bool {
	r, _ := utf8DecodeRune(text)
	if r == 0 {
		return false
	}
	return unicode.IsUpper(r) || unicode.IsDigit(r)
}

func updateHeadingStack(stack []headingFrame, block textBlock) []headingFrame {
	level := block.HeadingLevel
	if level <= 0 {
		level = 1
	}
	if isGenericHeading(block.Text) {
		return stack
	}

	for len(stack) > 0 && level <= stack[len(stack)-1].Level {
		stack = stack[:len(stack)-1]
	}

	return append(stack, headingFrame{
		Text:  block.Text,
		Level: level,
	})
}

func currentHeadingInfo(stack []headingFrame) (string, int) {
	if len(stack) == 0 {
		return "", 0
	}
	last := stack[len(stack)-1]
	return last.Text, last.Level
}

func sectionPath(stack []headingFrame) string {
	if len(stack) == 0 {
		return ""
	}

	parts := make([]string, 0, len(stack))
	for _, item := range stack {
		parts = append(parts, item.Text)
	}
	return strings.Join(parts, " > ")
}

func renderChunkContent(stack []headingFrame, body []string) string {
	parts := make([]string, 0, len(stack)+len(body))
	for _, item := range stack {
		parts = append(parts, item.Text)
	}
	parts = append(parts, body...)
	return strings.Join(parts, "\n\n")
}

func splitOversizedBlock(block textBlock) []string {
	if len(block.Text) <= maxBlockChars {
		return []string{block.Text}
	}

	var units []string
	switch block.Kind {
	case blockList, blockTable:
		units = splitByNewlineUnits(block.Text)
	default:
		units = splitIntoSentences(block.Text)
	}
	if len(units) == 0 {
		return []string{block.Text}
	}

	pieces := make([]string, 0, len(units))
	current := make([]string, 0, 8)

	flush := func() {
		if len(current) == 0 {
			return
		}
		pieces = append(pieces, strings.Join(current, unitSeparator(block.Kind)))
		current = current[:0]
	}

	for _, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		if len(unit) > maxBlockChars {
			flush()
			pieces = append(pieces, splitByLength(unit, maxBlockChars)...)
			continue
		}

		candidate := append(append([]string{}, current...), unit)
		if len(strings.Join(candidate, unitSeparator(block.Kind))) > maxBlockChars && len(current) > 0 {
			flush()
		}
		current = append(current, unit)
	}

	flush()

	if len(pieces) == 0 {
		return []string{block.Text}
	}

	return pieces
}

func splitByNewlineUnits(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitIntoSentences(text string) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	sentences := make([]string, 0, 8)
	start := 0
	for i, r := range runes {
		if r != '.' && r != '!' && r != '?' && r != ';' {
			continue
		}

		next := i + 1
		for next < len(runes) && unicode.IsSpace(runes[next]) {
			next++
		}
		if next < len(runes) && !unicode.IsUpper(runes[next]) && !unicode.IsDigit(runes[next]) {
			continue
		}

		part := strings.TrimSpace(string(runes[start : i+1]))
		if part != "" {
			sentences = append(sentences, part)
		}
		start = next
	}

	if start < len(runes) {
		rest := strings.TrimSpace(string(runes[start:]))
		if rest != "" {
			sentences = append(sentences, rest)
		}
	}

	if len(sentences) <= 1 {
		return splitByLength(text, maxBlockChars)
	}

	return sentences
}

func splitByLength(text string, limit int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= limit {
		return []string{text}
	}

	parts := make([]string, 0, (len(text)/limit)+1)
	remaining := text
	for len(remaining) > limit {
		cut := limit
		for cut > limit/2 && cut < len(remaining) && !unicode.IsSpace(rune(remaining[cut])) {
			cut--
		}
		if cut <= limit/2 {
			cut = limit
		}
		part := strings.TrimSpace(remaining[:cut])
		if part != "" {
			parts = append(parts, part)
		}
		remaining = strings.TrimSpace(remaining[cut:])
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}

func unitSeparator(kind blockKind) string {
	if kind == blockList || kind == blockTable {
		return "\n"
	}
	return " "
}

func normalizeHeadingText(text string) string {
	trimmed := strings.TrimSpace(text)
	if matches := markdownHeadingRegex.FindStringSubmatch(trimmed); len(matches) == 3 {
		return strings.TrimSpace(matches[2])
	}
	return trimmed
}

func isGenericHeading(text string) bool {
	return genericHeadingRegex.MatchString(strings.TrimSpace(text))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func chunkID(docID DocumentID, chunkIndex int) string {
	return string(docID) + ":" + strconvItoa(chunkIndex)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func strconvItoa(v int) string {
	return strconv.FormatInt(int64(v), 10)
}
