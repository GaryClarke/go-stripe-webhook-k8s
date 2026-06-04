package main

// Log message names (M6 contract). See docs/branches/14-structured-logging.md.
const (
	msgRequestStarted   = "request_started"
	msgRequestCompleted = "request_completed"

	msgStripeEventAccepted     = "stripe_event_accepted"
	msgStripeEventVerifyFailed = "stripe_event_verify_failed"
	msgStripeBodyTooLarge      = "stripe_body_too_large"

	msgProbeEncodeError = "probe_encode_error"
	msgPanic            = "panic"

	msgConfigLoadFailed    = "config_load_failed"
	msgServerListening     = "server_listening"
	msgServerListenError   = "server_listen_error"
	msgShuttingDown        = "shutting_down"
	msgServerShutdownError = "server_shutdown_error"
)
