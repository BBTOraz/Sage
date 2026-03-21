package chunking

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	passportTokenRegex   = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}\-_]{1,}`)
	acronymPattern       = regexp.MustCompile(`([A-Za-z][A-Za-z0-9/\- ]{3,}?)\s+\(([A-Z][A-Z0-9]{1,10})\)`)
	urlPattern           = regexp.MustCompile(`https?://`)
	isbnPattern          = regexp.MustCompile(`(?i)\bISBN\b`)
	englishStopwords     = toWordSet([]string{"the", "and", "for", "with", "that", "this", "from", "into", "using", "will", "have", "has", "are", "was", "were", "about", "into", "through", "their", "there", "only", "into", "http", "https", "www", "com", "org", "docs"})
	russianStopwords     = toWordSet([]string{"это", "для", "что", "как", "при", "его", "ее", "она", "они", "или", "все", "быть", "так", "над", "под", "при", "после", "если", "также", "документ", "ресурс"})
)

type scoredItem struct {
	Value string
	Score int
}

func buildPassport(content string, blocks []textBlock) DocumentPassport {
	passport := DocumentPassport{
		Language: detectLanguage(content),
	}

	passport.DocumentType = detectDocumentType(content, blocks)
	passport.TopTerms = extractTopTerms(content, passport.Language, 12)
	passport.KeyPhrases = extractKeyPhrases(blocks, 8)
	passport.Acronyms, passport.Aliases = extractAcronymsAndAliases(content)

	return passport
}

func detectLanguage(content string) string {
	var latin, cyrillic int
	for _, r := range content {
		switch {
		case unicode.In(r, unicode.Latin):
			latin++
		case unicode.In(r, unicode.Cyrillic):
			cyrillic++
		}
	}

	total := latin + cyrillic
	if total == 0 {
		return "unknown"
	}

	latinRatio := float64(latin) / float64(total)
	cyrillicRatio := float64(cyrillic) / float64(total)

	switch {
	case latinRatio >= 0.7:
		return "en"
	case cyrillicRatio >= 0.7:
		return "ru"
	case latinRatio >= 0.25 && cyrillicRatio >= 0.25:
		return "mixed"
	default:
		return "unknown"
	}
}

func detectDocumentType(content string, blocks []textBlock) string {
	listBlocks := 0
	headingBlocks := 0
	for _, block := range blocks {
		switch block.Kind {
		case blockList:
			listBlocks++
		case blockHeading:
			headingBlocks++
		}
	}

	urlCount := len(urlPattern.FindAllStringIndex(content, -1))
	isbnCount := len(isbnPattern.FindAllStringIndex(content, -1))
	if headingBlocks >= 2 && listBlocks < 3 {
		return "guide"
	}
	if listBlocks >= 2 || urlCount >= 2 || isbnCount >= 2 || strings.Count(content, "URL:") >= 2 {
		return "reference"
	}
	return "general"
}

func extractTopTerms(content, language string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	stopwords := englishStopwords
	if language == "ru" {
		stopwords = russianStopwords
	}

	frequencies := make(map[string]int)
	for _, token := range passportTokenRegex.FindAllString(strings.ToLower(content), -1) {
		if len(token) < 3 || stopwords[token] {
			continue
		}
		if isMostlyDigits(token) {
			continue
		}
		frequencies[token]++
	}

	if len(frequencies) == 0 {
		return nil
	}

	scored := make([]scoredItem, 0, len(frequencies))
	for token, score := range frequencies {
		scored = append(scored, scoredItem{Value: token, Score: score})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Value < scored[j].Value
		}
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	out := make([]string, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.Value)
	}
	return out
}

func extractKeyPhrases(blocks []textBlock, limit int) []string {
	if limit <= 0 {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, limit)
	for _, block := range blocks {
		if block.Kind != blockHeading {
			continue
		}
		phrase := canonicalPhrase(normalizeHeadingText(block.Text))
		if phrase == "" || isGenericHeading(phrase) {
			continue
		}
		if _, ok := seen[phrase]; ok {
			continue
		}
		out = append(out, phrase)
		seen[phrase] = struct{}{}
		if len(out) == limit {
			break
		}
	}
	return out
}

func canonicalPhrase(text string) string {
	text = strings.TrimSpace(text)
	if matches := acronymPattern.FindStringSubmatch(text); len(matches) == 3 && strings.EqualFold(strings.TrimSpace(matches[0]), text) {
		return strings.TrimSpace(matches[1])
	}
	return text
}

func extractAcronymsAndAliases(content string) ([]string, []string) {
	acronymSeen := make(map[string]struct{})
	aliasSeen := make(map[string]struct{})
	acronyms := make([]string, 0, 8)
	aliases := make([]string, 0, 8)

	for _, match := range acronymPattern.FindAllStringSubmatch(content, -1) {
		if len(match) != 3 {
			continue
		}
		longForm := strings.TrimSpace(match[1])
		acronym := strings.TrimSpace(match[2])
		if len(longForm) < 4 || len(acronym) < 2 {
			continue
		}
		if _, ok := acronymSeen[acronym]; !ok {
			acronyms = append(acronyms, acronym)
			acronymSeen[acronym] = struct{}{}
		}
		if _, ok := aliasSeen[longForm]; !ok {
			aliases = append(aliases, longForm)
			aliasSeen[longForm] = struct{}{}
		}
	}

	return acronyms, aliases
}

func toWordSet(words []string) map[string]bool {
	out := make(map[string]bool, len(words))
	for _, word := range words {
		out[word] = true
	}
	return out
}

func isMostlyDigits(token string) bool {
	digits := 0
	for _, r := range token {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits == len([]rune(token))
}
