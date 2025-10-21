# List of projects to provide to the make-docs script.
# Format is PROJECT[:[VERSION][:[REPOSITORY][:[DIRECTORY]]]]
PROJECTS := grafana::$(notdir $(basename $(shell git rev-parse --show-toplevel)../grafana)) \
	arbitrary:$(shell git rev-parse --show-toplevel)/docs/sources:/hugo/content/docs/grafana/latest/datasources/aws-cloudwatch \
