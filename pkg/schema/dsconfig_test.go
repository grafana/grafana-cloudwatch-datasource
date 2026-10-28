package schema_test

import (
	_ "embed"
	"testing"

	"github.com/grafana/dsconfig/schema"

	"github.com/grafana/grafana-cloudwatch-datasource/pkg/cloudwatch/models"
)

//go:embed dsconfig.json
var configSchemaJSON []byte

// settingsJSONModel extends models.CloudWatchSettings for the conformance suite
// without changing the runtime struct:
//   - AssumeRoleARN is shadowed with the schema's assumeRoleArn tag (awsds tags it
//     assumeRoleARN; both load at runtime via encoding/json's case-insensitive match).
//   - LogsTimeout is shadowed with a string, since the runtime Duration type has a
//     custom UnmarshalJSON the type check can't see through.
//   - LogGroups, DefaultLogGroups and TracingDatasourceUID are frontend-only.
type settingsJSONModel struct {
	models.CloudWatchSettings
	AssumeRoleARN        string   `json:"assumeRoleArn"`
	LogsTimeout          string   `json:"logsTimeout"`
	LogGroups            []any    `json:"logGroups"`
	DefaultLogGroups     []string `json:"defaultLogGroups"`
	TracingDatasourceUID string   `json:"tracingDatasourceUid"`
}

//go:generate go test -run TestPlugin -generateArtifacts
func TestPlugin(t *testing.T) {
	schema.RunPluginTests(t, schema.PluginUnderTest{
		ID:                "cloudwatch",
		ConfigSchemaJSON:  configSchemaJSON,
		SettingsJSONModel: settingsJSONModel{},
		SecureKeys:        []string{"accessKey", "secretKey", "sessionToken", "proxyPassword"},
	})
}
