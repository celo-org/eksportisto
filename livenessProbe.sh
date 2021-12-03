#!/bin/bash
# Default port for Prometheus is 9090
port=$1
processingTimeout=$2
# Look for success metric line at /metrics endpoint
targetMetric="eksportisto_last_processing_attempt"
match=$(curl --silent 'http://127.0.0.1:'$port'/metrics' | grep -E "^${targetMetric} \d*")
# Remove all but the value.
metricValue="${match/$targetMetric/}"

# Convert to a timestamp (integer).
lastTimestamp=$(printf "%.0f\n" $metricValue)
# Get the current time.
now=$(date -u +%s)
# Calculate how many seconds have elapsed since the last processing attempt.
timeSinceLastAttempt=$(($now - $lastTimestamp))

if [[ "$timeSinceLastAttempt" -lt "$processingTimeout" ]]; then
  echo "IT'S ALIVE!";
  exit 0;
fi

echo "RIP";
exit 1;

