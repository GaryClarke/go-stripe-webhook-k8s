package main

import (
	"context"
	"integration-engine/internal/config"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body:       `{"status":"ok"}`,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func main() {
	if _, err := config.Load(); err != nil {
		log.Fatalf("config: %v", err)
	}
	lambda.Start(handler)
}
