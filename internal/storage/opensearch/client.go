package opensearch

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type Client struct {
	Config Config
	API    *opensearchapi.Client
}

type Info struct {
	ClusterName string
	ClusterUUID string
	Version     string
	Tagline     string
}

type HealthChecker interface {
	Ping(ctx context.Context) (Info, error)
}

func (c *Client) Ping(ctx context.Context) (Info, error) {
	resp, err := c.API.Info(ctx, nil)
	if err != nil {
		return Info{}, err
	}
	return Info{
		ClusterName: resp.ClusterName,
		ClusterUUID: resp.ClusterUUID,
		Version:     resp.Version.Number,
		Tagline:     resp.Tagline,
	}, nil
}

func NewClient(cfg Config) (*Client, error) {
	var tlsConfig *tls.Config
	if cfg.UseTLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}
	}

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: cfg.Addresses,
			Username:  cfg.Username,
			Password:  cfg.Password,
			Transport: &http.Transport{
				ResponseHeaderTimeout: cfg.RequestTimeout,
				TLSClientConfig:       tlsConfig,
			},
		},
	})

	if err != nil {
		return nil, err
	}

	return &Client{
		Config: cfg,
		API:    client,
	}, nil
}
