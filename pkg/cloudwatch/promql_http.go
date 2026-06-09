package cloudwatch

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func (ds *DataSource) promqlSignedGet(ctx context.Context, region, path string, params url.Values, timeout time.Duration) ([]byte, int, error) {
	rawURL := fmt.Sprintf("https://monitoring.%s.amazonaws.com%s", region, path)
	if len(params) > 0 {
		rawURL += "?" + params.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %w", err)
	}

	cfg, err := ds.newAWSConfig(ctx, region)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get AWS config: %w", err)
	}

	credentials, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, 0, backend.DownstreamError(fmt.Errorf("failed to retrieve credentials: %w", err))
	}

	if err := v4.NewSigner().SignHTTP(ctx, credentials, httpReq, fmt.Sprintf("%x", sha256.Sum256(nil)), "monitoring", region, time.Now().UTC()); err != nil {
		return nil, 0, fmt.Errorf("failed to sign request: %w", err)
	}

	httpResp, err := (&http.Client{Timeout: timeout}).Do(httpReq)
	if err != nil {
		return nil, 0, backend.DownstreamError(fmt.Errorf("request failed: %w", err))
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	return body, httpResp.StatusCode, nil
}
