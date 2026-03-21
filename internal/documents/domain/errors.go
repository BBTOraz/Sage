package domain

import "errors"

var (
	ErrDocumentNotFound     = errors.New("document not found")
	ErrChunkNotFound        = errors.New("chunk not found")
	ErrInsufficientEvidence = errors.New("insufficient evidence")
)
