package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type ExternalIdResponse struct {
	ExternalId string `json:"externalId"`
}

func ExternalIdHandler(ctx context.Context, pluginCtx backend.PluginContext, reqCtxBeforeAuth models.RequestContextFactoryFunc, parameters url.Values) ([]byte, *models.HttpError) {
	reqCtx, err := reqCtxBeforeAuth(ctx, pluginCtx, "")
	if err != nil {
		return nil, models.NewHttpError("error in ExternalIdHandler", http.StatusInternalServerError, err)
	}

	response := ExternalIdResponse{
		ExternalId: reqCtx.Settings.GrafanaSettings.ExternalID,
	}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return nil, models.NewHttpError("error in ExternalIdHandler", http.StatusInternalServerError, err)
	}

	return jsonResponse, nil
}
