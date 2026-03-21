package pipeline

import (
	"bilge-lib/internal/ingestion/chunking"
	"bilge-lib/internal/ingestion/loader"
	parsepkg "bilge-lib/internal/ingestion/parser"
	storage "bilge-lib/internal/storage/opensearch"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrNilFileDescriptor = errors.New("file descriptor is nil")

type FileParser interface {
	Parse(ctx context.Context, file loader.FileDescriptor) (*parsepkg.ParsedFile, error)
}

type LocalIngester struct {
	Surveyor    loader.Surveyor
	Parser      FileParser
	Transformer chunking.Transformer
	Indexes     storage.IndexEnsurer
	Indexer     storage.ChunkIndexer
	StatusStore storage.StatusWriter
	workers     int
	now         func() time.Time
}

type pendingIndexedFile struct {
	result  *FileIngestResult
	records []storage.ChunkIndexRecord
	position int
}

type fileWorkResult struct {
	result   *FileIngestResult
	records  []storage.ChunkIndexRecord
	position int
	err      error
}

func NewLocalIngester(
	surveyor loader.Surveyor,
	parser FileParser,
	transformer chunking.Transformer,
	indexes storage.IndexEnsurer,
	indexer storage.ChunkIndexer,
	statusStore storage.StatusWriter,
) *LocalIngester {
	return &LocalIngester{
		Surveyor:    surveyor,
		Parser:      parser,
		Transformer: transformer,
		Indexes:     indexes,
		Indexer:     indexer,
		StatusStore: statusStore,
		workers:     1,
		now:         time.Now,
	}
}

func (i *LocalIngester) WithWorkers(workers int) *LocalIngester {
	if workers < 1 {
		workers = 1
	}
	i.workers = workers
	return i
}

func (i *LocalIngester) Ingest(ctx context.Context, root string) (*IngestReport, error) {
	startedAt := time.Now()
	surveyStartedAt := time.Now()
	descriptors, err := i.Surveyor.Survey(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("survey directory: %w", err)
	}

	report := &IngestReport{
		RunID: makeRunID(root, i.now()),
		Root:  root,
		Files: make([]FileIngestResult, 0, len(descriptors)),
	}
	report.Timings.SurveyMS = durationMS(surveyStartedAt)

	ensureIndexesStartedAt := time.Now()
	if err := i.Indexes.EnsureIndexes(ctx); err != nil {
		report.Timings.EnsureIndexesMS = durationMS(ensureIndexesStartedAt)
		report.Timings.TotalMS = durationMS(startedAt)
		return report, fmt.Errorf("ensure ingest indexes: %w", err)
	}
	report.Timings.EnsureIndexesMS = durationMS(ensureIndexesStartedAt)

	filesStartedAt := time.Now()
	pending := make([]pendingIndexedFile, 0, len(descriptors))
	for _, work := range i.processFiles(ctx, report.RunID, descriptors) {
		result, records, err := work.result, work.records, work.err
		if result != nil {
			if len(records) != 0 {
				report.Files = append(report.Files, *result)
				pending = append(pending, pendingIndexedFile{
					result:   result,
					records:  records,
					position: work.position,
				})
			} else {
				report.Files = append(report.Files, *result)
				if result.Status == FileStatusIndexed {
					report.IndexedDocuments++
					report.IndexedChunks += result.ChunkCount
				}
			}
		}
		if err != nil {
			report.Timings.FilesMS = durationMS(filesStartedAt)
			report.Timings.TotalMS = durationMS(startedAt)
			return report, err
		}
	}
	if err := i.flushPending(ctx, report.RunID, pending, report); err != nil {
		report.Timings.FilesMS = durationMS(filesStartedAt)
		report.Timings.TotalMS = durationMS(startedAt)
		return report, err
	}
	report.Timings.FilesMS = durationMS(filesStartedAt)
	report.Timings.TotalMS = durationMS(startedAt)

	return report, nil
}

func (i *LocalIngester) ingestFile(ctx context.Context, runID string, descriptor *loader.FileDescriptor) (*FileIngestResult, []storage.ChunkIndexRecord, error) {
	if descriptor == nil {
		return nil, nil, ErrNilFileDescriptor
	}

	result := &FileIngestResult{
		Path:     descriptor.Path,
		FileType: descriptor.FileType,
	}
	fileStartedAt := time.Now()

	parseStartedAt := time.Now()
	parsed, err := i.Parser.Parse(ctx, *descriptor)
	result.Timings.ParseMS = durationMS(parseStartedAt)
	if err != nil {
		result.Status = FileStatusFailed
		result.Stage = FileStageParse
		result.Error = err.Error()
		err = i.writeStatus(ctx, runID, result)
		result.Timings.TotalMS = durationMS(fileStartedAt)
		return result, nil, err
	}

	result.Warnings = cloneStrings(parsed.Warnings)
	if parsed.Status == parsepkg.ParseStatusUntrusted {
		result.Status = FileStatusSkipped
		result.Stage = FileStageParse
		err = i.writeStatus(ctx, runID, result)
		result.Timings.TotalMS = durationMS(fileStartedAt)
		return result, nil, err
	}

	transformStartedAt := time.Now()
	chunked, err := i.Transformer.Transform(ctx, parsed)
	result.Timings.TransformMS = durationMS(transformStartedAt)
	if err != nil {
		result.Status = FileStatusFailed
		result.Stage = FileStageTransform
		result.Error = err.Error()
		err = i.writeStatus(ctx, runID, result)
		result.Timings.TotalMS = durationMS(fileStartedAt)
		return result, nil, err
	}

	result.DocumentID = string(chunked.SourceDocument.ID)
	records := storage.MapChunkedFileToChunkRecords(chunked, len(parsed.Warnings), i.now().UTC())
	result.ChunkCount = len(records)
	return result, records, nil
}

func (i *LocalIngester) processFiles(ctx context.Context, runID string, descriptors []*loader.FileDescriptor) []fileWorkResult {
	results := make([]fileWorkResult, len(descriptors))
	if len(descriptors) == 0 {
		return results
	}

	type job struct {
		position   int
		descriptor *loader.FileDescriptor
	}

	jobs := make(chan job)
	var wg sync.WaitGroup
	workerCount := i.workers
	if workerCount > len(descriptors) {
		workerCount = len(descriptors)
	}

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				result, records, err := i.ingestFile(ctx, runID, job.descriptor)
				results[job.position] = fileWorkResult{
					result:   result,
					records:  records,
					position: job.position,
					err:      err,
				}
			}
		}()
	}

	for position, descriptor := range descriptors {
		jobs <- job{
			position:   position,
			descriptor: descriptor,
		}
	}
	close(jobs)
	wg.Wait()

	return results
}

func (i *LocalIngester) writeStatus(ctx context.Context, runID string, result *FileIngestResult) error {
	statusStartedAt := time.Now()
	record := storage.IngestStatusRecord{
		RunID:      runID,
		Path:       result.Path,
		FileType:   string(result.FileType),
		Status:     string(result.Status),
		Stage:      string(result.Stage),
		Warnings:   cloneStrings(result.Warnings),
		Error:      result.Error,
		DocumentID: result.DocumentID,
		ChunkCount: result.ChunkCount,
		RecordedAt: i.now().UTC(),
	}

	if err := i.StatusStore.WriteStatus(ctx, record); err != nil {
		result.Timings.StatusWriteMS = durationMS(statusStartedAt)
		return fmt.Errorf("write ingest status for %q: %w", result.Path, err)
	}
	result.Timings.StatusWriteMS = durationMS(statusStartedAt)

	return nil
}

func makeRunID(root string, now time.Time) string {
	sum := sha256.Sum256([]byte(root + ":" + now.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:8])
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	out := make([]string, len(in))
	copy(out, in)
	return out
}

func durationMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func (i *LocalIngester) flushPending(ctx context.Context, runID string, pending []pendingIndexedFile, report *IngestReport) error {
	if len(pending) == 0 {
		return nil
	}

	records := make([]storage.ChunkIndexRecord, 0)
	for _, item := range pending {
		records = append(records, item.records...)
	}

	indexStartedAt := time.Now()
	indexErr := i.Indexer.IndexChunks(ctx, records)
	indexMS := durationMS(indexStartedAt)

	for _, item := range pending {
		item.result.Timings.IndexMS = indexMS
		if indexErr != nil {
			item.result.Status = FileStatusFailed
			item.result.Stage = FileStageIndex
			item.result.Error = indexErr.Error()
		} else {
			item.result.Status = FileStatusIndexed
			item.result.Stage = FileStageIndex
		}

		if err := i.writeStatus(ctx, runID, item.result); err != nil {
			item.result.Timings.TotalMS = item.result.Timings.ParseMS + item.result.Timings.TransformMS + item.result.Timings.IndexMS + item.result.Timings.StatusWriteMS
			report.Files[item.position] = *item.result
			return err
		}
		item.result.Timings.TotalMS = item.result.Timings.ParseMS + item.result.Timings.TransformMS + item.result.Timings.IndexMS + item.result.Timings.StatusWriteMS

		report.Files[item.position] = *item.result
		if item.result.Status == FileStatusIndexed {
			report.IndexedDocuments++
			report.IndexedChunks += item.result.ChunkCount
		}
	}

	return nil
}
