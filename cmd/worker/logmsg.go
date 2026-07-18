package main

// Log message names (M6 contract). See docs/branches/14-structured-logging.md.
const (
	configLoadFailed = "config_load_failed"
	storeOpenFailed  = "store_open_failed"

	workerStarted = "worker_started"
	shuttingDown  = "shutting_down"

	kafkaClientFailed = "kafka_client_failed"
	kafkaFetchFailed  = "kafka_fetch_failed"
	kafkaCommitFailed = "kafka_commit_failed"

	stripeJobConsumed        = "stripe_job_consumed"
	stripeJobUnmarshalFailed = "stripe_job_unmarshal_failed"
)
