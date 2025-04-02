package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/routes"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func (ds *DataSource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/ebs-volume-ids", handleResourceReq(ds.handleGetEbsVolumeIds, ds.logger))
	mux.HandleFunc("/ec2-instance-attribute", handleResourceReq(ds.handleGetEc2InstanceAttribute, ds.logger))
	mux.HandleFunc("/resource-arns", handleResourceReq(ds.handleGetResourceArns, ds.logger))
	mux.HandleFunc("/log-groups", routes.ResourceRequestMiddleware(routes.LogGroupsHandler, ds.logger, ds.getRequestContext))
	mux.HandleFunc("/metrics", routes.ResourceRequestMiddleware(routes.MetricsHandler, ds.logger, ds.getRequestContext))
	mux.HandleFunc("/dimension-values", routes.ResourceRequestMiddleware(routes.DimensionValuesHandler, ds.logger, ds.getRequestContext))
	mux.HandleFunc("/dimension-keys", routes.ResourceRequestMiddleware(routes.DimensionKeysHandler, ds.logger, ds.getRequestContext))
	mux.HandleFunc("/accounts", routes.ResourceRequestMiddleware(routes.AccountsHandler, ds.logger, ds.getRequestContext))
	mux.HandleFunc("/namespaces", routes.ResourceRequestMiddleware(routes.NamespacesHandler, ds.logger, ds.getRequestContext))
	mux.HandleFunc("/log-group-fields", routes.ResourceRequestMiddleware(routes.LogGroupFieldsHandler, ds.logger, ds.getRequestContext))
	mux.HandleFunc("/external-id", routes.ResourceRequestMiddleware(routes.ExternalIdHandler, ds.logger, ds.getRequestContextOnlySettings))
	mux.HandleFunc("/regions", routes.ResourceRequestMiddleware(routes.RegionsHandler, ds.logger, ds.getRequestContext))
	// remove this once AWS's Cross Account Observability is supported in GovCloud
	mux.HandleFunc("/legacy-log-groups", handleResourceReq(ds.handleGetLogGroups, ds.logger))

	return mux
}

type handleFn func(ctx context.Context, parameters url.Values) ([]suggestData, error)

func handleResourceReq(handleFunc handleFn, logger log.Logger) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		err := req.ParseForm()
		if err != nil {
			writeResponse(rw, http.StatusBadRequest, fmt.Sprintf("unexpected error %v", err), logger.FromContext(ctx))
			return
		}
		data, err := handleFunc(ctx, req.URL.Query())
		if err != nil {
			writeResponse(rw, http.StatusBadRequest, fmt.Sprintf("unexpected error %v", err), logger.FromContext(ctx))
			return
		}
		body, err := json.Marshal(data)
		if err != nil {
			writeResponse(rw, http.StatusBadRequest, fmt.Sprintf("unexpected error %v", err), logger.FromContext(ctx))
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, err = rw.Write(body)
		if err != nil {
			logger.FromContext(ctx).Error("Unable to write HTTP response", "error", err)
			return
		}
	}
}

func writeResponse(rw http.ResponseWriter, code int, msg string, logger log.Logger) {
	rw.WriteHeader(code)
	_, err := rw.Write([]byte(msg))
	if err != nil {
		logger.Error("Unable to write HTTP response", "error", err)
	}
}
