package cloudwatch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/grafana/grafana-aws-sdk/pkg/awsauth"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

func (ds *DataSource) promqlSignedGet(ctx context.Context, region, path string, params url.Values, timeout time.Duration) ([]byte, int, error) {
	if region == defaultRegion || region == "" {
		region = ds.Settings.Region
	}

	baseURL := ds.Settings.Endpoint
	if baseURL == "" {
		endpoint, err := cloudwatch.NewDefaultEndpointResolver().ResolveEndpoint(region, cloudwatch.EndpointResolverOptions{})
		if err != nil {
			return nil, 0, backend.DownstreamError(fmt.Errorf("failed to resolve CloudWatch endpoint: %w", err))
		}
		baseURL = endpoint.URL
	}
	rawURL := strings.TrimRight(baseURL, "/") + path

	if len(params) > 0 {
		rawURL += "?" + params.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %w", err)
	}

	authType := ds.Settings.AuthType
	timeouts := httpclient.DefaultTimeoutOptions
	timeouts.Timeout = timeout

	opts := httpclient.Options{
		Timeouts: &timeouts,
		SigV4: &httpclient.SigV4Config{
			AuthType:      authType.String(),
			Profile:       ds.Settings.Profile,
			Service:       "monitoring",
			AccessKey:     ds.Settings.AccessKey,
			SecretKey:     ds.Settings.SecretKey,
			SessionToken:  ds.Settings.SessionToken,
			AssumeRoleARN: ds.Settings.AssumeRoleARN,
			ExternalID:    ds.Settings.ExternalID,
			Region:        region,
		},
		Middlewares: append(httpclient.DefaultMiddlewares(), awsauth.NewSigV4Middleware()),
	}

	if ds.Settings.GrafanaSettings.SecureSocksDSProxyEnabled && ds.Settings.SecureSocksProxyEnabled {
		opts.ProxyOptions = ds.ProxyOpts
	}

	client, err := NewPromQLHTTPClient(opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build PromQL HTTP client: %w", err)
	}

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, backend.DownstreamError(fmt.Errorf("request failed: %w", err))
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, 0, backend.DownstreamError(fmt.Errorf("failed to read response body: %w", err))
	}
	return body, httpResp.StatusCode, nil
}
