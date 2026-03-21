package session

import (
	"bilge-lib/internal/documents/domain"
	"bilge-lib/internal/documents/ports"
)

type RequestState struct {
	UserQuestion string

	ActiveQueryID domain.QueryID
	NextCursor    string

	Shortlist     []ports.SearchHit
	OpenedWindows []ports.ChunkWindow

	Evidence domain.EvidenceBundle
	Gaps     []string
}

func (s *RequestState) ApplySearchPage(page ports.SearchPage) {
	s.ActiveQueryID = page.QueryID
	s.NextCursor = page.NextCursor
	s.Shortlist = append([]ports.SearchHit(nil), page.Items...)
}

func (s *RequestState) RememberWindow(window ports.ChunkWindow) {
	s.OpenedWindows = append(s.OpenedWindows, window)
}

func (s *RequestState) SetEvidence(bundle domain.EvidenceBundle) {
	s.Evidence = bundle
	s.Gaps = s.Gaps[:0]
	for _, gap := range bundle.Gaps {
		s.Gaps = append(s.Gaps, gap.Reason)
	}
}
