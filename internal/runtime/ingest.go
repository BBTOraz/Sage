package runtime

import (
	"bilge-lib/internal/ingestion/pipeline"
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type IngestStatus string
type IngestEventType string
type IngestID string

const (
	IngestStatusQueued    IngestStatus = "queued"
	IngestStatusRunning   IngestStatus = "running"
	IngestStatusCompleted IngestStatus = "completed"
	IngestStatusFailed    IngestStatus = "failed"
)

const (
	EventIngestQueued    IngestEventType = "ingest_queued"
	EventIngestStarted   IngestEventType = "ingest_started"
	EventIngestCompleted IngestEventType = "ingest_completed"
	EventIngestFailed    IngestEventType = "ingest_failed"
)

type Ingestor interface {
	Ingest(ctx context.Context, root string) (*pipeline.IngestReport, error)
}

type IngestEvent struct {
	JobID      IngestID
	Root       string
	Status     IngestStatus
	Type       IngestEventType
	Report     *pipeline.IngestReport
	Err        error
	OccurredAt time.Time
}

type IngestHandle struct {
	ID     IngestID
	Status IngestStatus
	Events <-chan IngestEvent
}

type ingestJob struct {
	id     IngestID
	root   string
	ctx    context.Context
	events chan IngestEvent
}

func validateIngestRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("directory path is required")
	}

	clean := filepath.Clean(root)
	info, err := os.Stat(clean)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("ingest path must be a directory")
	}

	return clean, nil
}

func (m *Manager) StartIngest(ctx context.Context, root string) (IngestHandle, error) {
	if m.ingester == nil {
		return IngestHandle{}, errors.New("ingest pipeline is not configured")
	}

	cleanRoot, err := validateIngestRoot(root)
	if err != nil {
		return IngestHandle{}, err
	}

	jobID := IngestID(uuid.New().String())
	events := make(chan IngestEvent, 8)
	job := ingestJob{
		id:     jobID,
		root:   cleanRoot,
		ctx:    ctx,
		events: events,
	}

	events <- IngestEvent{
		JobID:      jobID,
		Root:       cleanRoot,
		Status:     IngestStatusQueued,
		Type:       EventIngestQueued,
		OccurredAt: time.Now(),
	}

	m.startIngestWorkers()
	m.ingestQueue <- job

	return IngestHandle{
		ID:     jobID,
		Status: IngestStatusQueued,
		Events: events,
	}, nil
}

func (m *Manager) startIngestWorkers() {
	m.ingestOnce.Do(func() {
		workerCount := m.ingestWorkers
		if workerCount < 1 {
			workerCount = 1
		}
		for i := 0; i < workerCount; i++ {
			go m.runIngestWorker()
		}
	})
}

func (m *Manager) runIngestWorker() {
	for job := range m.ingestQueue {
		job.events <- IngestEvent{
			JobID:      job.id,
			Root:       job.root,
			Status:     IngestStatusRunning,
			Type:       EventIngestStarted,
			OccurredAt: time.Now(),
		}

		report, err := m.ingester.Ingest(job.ctx, job.root)
		status := IngestStatusCompleted
		eventType := EventIngestCompleted
		if err != nil {
			status = IngestStatusFailed
			eventType = EventIngestFailed
		}

		job.events <- IngestEvent{
			JobID:      job.id,
			Root:       job.root,
			Status:     status,
			Type:       eventType,
			Report:     report,
			Err:        err,
			OccurredAt: time.Now(),
		}
		close(job.events)
	}
}
