package main

import "errors"

type JobError struct {
	Msg        string
	Retryable  bool
	HTTPStatus int
	Cause      error
}

func (e *JobError) Error() string {
	if e.Cause != nil {
		return e.Msg + ": " + e.Cause.Error()
	}
	return e.Msg
}

func (e *JobError) IsRetryable() bool { return e.Retryable }

func isRetryableJobError(err error) bool {
	if je, ok := errors.AsType[*JobError](err); ok {
		return je.Retryable
	}
	return true // unknown errors: retry (safe default until classified)
}

func classifyHTTPStatus(status int) error {
	switch {
	case status >= 500, status == 429:
		return &JobError{Msg: "downstream error", Retryable: true, HTTPStatus: status}
	case status == 400, status == 404, status == 422:
		return &JobError{Msg: "downstream rejected job", Retryable: false, HTTPStatus: status}
	default:
		// conservative: unknown 4xx → permanent (don't stall partition)
		return &JobError{Msg: "downstream unexpected status", Retryable: false, HTTPStatus: status}
	}
}

func permanentJobError(msg string, cause error) *JobError {
	return &JobError{
		Msg:       msg,
		Retryable: false,
		Cause:     cause,
	}
}

func retryableJobError(msg string, cause error, httpStatus int) *JobError {
	return &JobError{
		Msg:        msg,
		Retryable:  true,
		HTTPStatus: httpStatus,
		Cause:      cause,
	}
}
