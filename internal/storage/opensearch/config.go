package opensearch

import "time"

// Config is the native OpenSearch connection and index configuration for Sage.
// TODO(user): wire this into app/core config loading once the client is implemented.
type Config struct {
	Addresses          []string
	ChunkIndexName     string
	StatusIndexName    string
	Username           string
	Password           string
	RequestTimeout     time.Duration
	UseTLS             bool
	InsecureSkipVerify bool
}

// DefaultConfig is the intended local-development baseline for Step 01.
// TODO(user): decide whether this should stay here or move into a higher-level config package.
var DefaultConfig = Config{
	Addresses:       []string{"http://localhost:9200"},
	ChunkIndexName:  "sage_chunks",
	StatusIndexName: "sage_ingest_status",
	RequestTimeout:  10 * time.Second,
}

func (c Config) ChunkIndex() string {
	if c.ChunkIndexName == "" {
		return DefaultConfig.ChunkIndexName
	}

	return c.ChunkIndexName
}

func (c Config) StatusIndex() string {
	if c.StatusIndexName == "" {
		return DefaultConfig.StatusIndexName
	}

	return c.StatusIndexName
}
