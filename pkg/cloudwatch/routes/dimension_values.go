package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models/resources"
)

func DimensionValuesHandler(ctx context.Context, reqCtxFactory models.RequestContextFactoryFunc, parameters url.Values) ([]byte, *models.HttpError) {
	dimensionValuesRequest, err := resources.GetDimensionValuesRequest(parameters)
	if err != nil {
		return nil, models.NewHttpError("error in DimensionValuesHandler", http.StatusBadRequest, err)
	}

	service, err := newListMetricsService(ctx, reqCtxFactory, dimensionValuesRequest.Region)
	if err != nil {
		return nil, models.NewHttpError("error in DimensionValuesHandler", http.StatusInternalServerError, err)
	}

	response, err := service.GetDimensionValuesByDimensionFilter(ctx, dimensionValuesRequest)
	if err != nil {
		return nil, models.NewHttpError("error in DimensionValuesHandler", http.StatusInternalServerError, err)
	}

	dimensionValuesResponse, err := json.Marshal(response)
	if err != nil {
		return nil, models.NewHttpError("error in DimensionValuesHandler", http.StatusInternalServerError, err)
	}

	return dimensionValuesResponse, nil
}
