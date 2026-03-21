package pipeline

import (
	"bilge-lib/internal/ingestion/loader"
	"context"
)

type FileStatus string
type FileStage string

const (
	FileStatusIndexed FileStatus = "indexed"
	FileStatusSkipped FileStatus = "skipped"
	FileStatusFailed  FileStatus = "failed"
)

const (
	FileStageSurvey    FileStage = "survey"
	FileStageParse     FileStage = "parse"
	FileStageTransform FileStage = "transform"
	FileStageIndex     FileStage = "index"
)

type IngestReport struct {
	RunID            string
	Root             string
	Files            []FileIngestResult
	IndexedDocuments int
	IndexedChunks    int
	Timings          IngestTimings
}

type FileIngestResult struct {
	Path       string
	FileType   loader.FileType
	Status     FileStatus
	Stage      FileStage
	Warnings   []string
	Error      string
	DocumentID string
	ChunkCount int
	Timings    FileTimings
}

type IngestTimings struct {
	SurveyMS       int64
	EnsureIndexesMS int64
	FilesMS        int64
	TotalMS        int64
}

type FileTimings struct {
	ParseMS       int64
	TransformMS   int64
	IndexMS       int64
	StatusWriteMS int64
	TotalMS       int64
}

type Ingester interface {
	Ingest(ctx context.Context, root string) (*IngestReport, error)
}
