package elaston

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	lambdaRunner "github.com/aws/aws-lambda-go/lambda"
	lambdaT "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	sqsT "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/davecgh/go-spew/spew"

	"github.com/bcap/elaston/aws"
	"github.com/bcap/elaston/deploy"
)

func Run(handler Handler) {
	if IsLambdaEnvironment() {
		runLambda(handler)
	} else {
		runTool()
	}
}

func IsLambdaEnvironment() bool {
	_, ok := os.LookupEnv("ELASTON_RUNNING_ON_LAMBDA")
	return ok
}

func lambdaFnName() string {
	return mustEnvVar("AWS_LAMBDA_FUNCTION_NAME")
}

func queueURL() string {
	return mustEnvVar("ELASTON_SQS_QUEUE_URL")
}

func mustEnvVar(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("env var %s not present", key))
	}
	return value
}

func runLambda(handler Handler) {
	elaston := New(aws.New(""), lambdaFnName(), queueURL())
	lambdaRunner.Start(lambdaHandler(elaston, handler))
}

func lambdaHandler(elaston *Elaston, handler Handler) func(context.Context, json.RawMessage) (any, error) {
	isSQSMessage := func(msg *sqsT.Message) bool {
		return msg != nil && msg.MessageId != nil && msg.MD5OfBody != nil && msg.Attributes != nil && msg.ReceiptHandle != nil
	}

	return func(ctx context.Context, rawPayload json.RawMessage) (any, error) {
		var payload interface{}

		// In case the function was invoked through SQS, unwrap the message payload
		sqsMessages := struct {
			Records []sqsT.Message
		}{}
		if err := json.Unmarshal(rawPayload, &sqsMessages); err == nil && len(sqsMessages.Records) > 0 && isSQSMessage(&sqsMessages.Records[0]) {
			rawPayload = []byte(*sqsMessages.Records[0].Body)
		}

		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			return nil, err
		}
		return handler.Handle(ctx, elaston, payload)
	}
}

func runTool() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	aws := aws.New("bcap")

	deployment, err := deploy.Deploy(ctx, aws, "elaston-test", programBytes(), 128)
	panicOnErr(err)

	stream, err := aws.InvokeLambdaFunction(ctx, *deployment.Function.Configuration.FunctionName, map[string]any{"a": 1})
	panicOnErr(err)

	for ev := range stream.Reader.Events() {
		switch v := ev.(type) {
		case *lambdaT.InvokeWithResponseStreamResponseEventMemberInvokeComplete:
			spew.Printf("lambda finished running: %v\n", v.Value)
		case *lambdaT.InvokeWithResponseStreamResponseEventMemberPayloadChunk:
			var stringValue string
			var jsonValue any
			if err := json.Unmarshal(v.Value.Payload, &jsonValue); err == nil {
				data, _ := json.MarshalIndent(jsonValue, "", "  ")
				stringValue = string(data)
			} else {
				stringValue = string(v.Value.Payload)
			}
			log.Printf("event from lambda: \n%v", stringValue)
		}
	}
}

func programBytes() []byte {
	f, err := os.OpenFile(os.Args[0], os.O_RDONLY, 0)
	panicOnErr(err)
	data, err := io.ReadAll(f)
	panicOnErr(err)
	return data
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func cleanup(ctx context.Context, deployment deploy.Deployment) {
	if err := deployment.Clean(ctx, true); err != nil {
		log.Printf("Failed to cleanup: %d errors: %v", len(err.Errors), err.Errors)
	}
}
