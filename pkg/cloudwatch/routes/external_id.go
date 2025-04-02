package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
)

type ExternalIdResponse struct {
	ExternalId string `json:"externalId"`
}

func ExternalIdHandler(ctx context.Context, reqCtxBeforeAuth models.RequestContextFactoryFunc, _ url.Values) ([]byte, *models.HttpError) {
	reqCtx, err := reqCtxBeforeAuth(ctx, "")
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
