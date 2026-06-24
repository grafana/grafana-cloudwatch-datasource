#!/bin/sh
# Seeds CloudWatch metric data into LocalStack for e2e tests.
#
# Window: FIXTURE_FROM_ISO ... FIXTURE_TO_ISO (4h, matches tests/e2e/queryEditor.spec.ts)
# Namespace: E2E/Demo, Metric: RequestCount, Dimensions: Service=api, Env=test
# Granularity: 1 datapoint per minute, deterministic pattern (no RNG).
set -e

ENDPOINT="${1:-http://ministack:4566}"
NS="E2E/Demo"
METRIC="RequestCount"
# 2026-04-21T00:00:00Z = 1776729600
START_TS=1776729600
# 4 hours = 240 minutes; one datapoint per minute
COUNT=240

i=0
# PutMetricData takes up to 1000 datapoints per call; batch in chunks of 20
# (each --metric-data item is independent so we just loop and let it retry)
while [ $i -lt $COUNT ]; do
  TS=$((START_TS + i * 60))
  # Deterministic value: 50 + 30*sin(i/10), ints only via shell arithmetic
  # shell can't do sin; approximate with a triangle wave 30..70..30 over 60 minutes
  CYCLE=$((i % 60))
  if [ $CYCLE -lt 30 ]; then
    VAL=$((30 + CYCLE))
  else
    VAL=$((90 - CYCLE))
  fi
  ISO=$(date -u -d "@$TS" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -r "$TS" +"%Y-%m-%dT%H:%M:%SZ")
  aws --endpoint-url="$ENDPOINT" cloudwatch put-metric-data \
    --namespace "$NS" \
    --metric-data "MetricName=$METRIC,Dimensions=[{Name=Service,Value=api},{Name=Env,Value=test}],Timestamp=$ISO,Value=$VAL,Unit=Count" \
    >/dev/null
  i=$((i + 1))
done

echo "Seeded $COUNT datapoints for $NS/$METRIC"
