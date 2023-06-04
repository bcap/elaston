package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaT "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func (aws *AWS) GetLambdaFunction(ctx context.Context, name string) (*lambda.GetFunctionOutput, error) {
	out, err := aws.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
	})
	if err != nil {
		var notFound *lambdaT.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (aws *AWS) InvokeLambdaFunction(ctx context.Context, name string, payload any) (*lambda.InvokeWithResponseStreamEventStream, error) {
	payloadBytes := []byte{}
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	out, err := aws.Lambda.InvokeWithResponseStream(ctx, &lambda.InvokeWithResponseStreamInput{
		FunctionName: &name,
		Payload:      payloadBytes,
	})
	if err != nil {
		return nil, err
	}

	return out.GetStream(), nil
}

func (aws *AWS) LambdaFunctionConsoleURL(functionName string) string {
	region := aws.Config.Region
	return fmt.Sprintf(
		"https://%s.console.aws.amazon.com/lambda/home?region=%s#/functions/%s?tab=monitoring",
		region, region, functionName,
	)
}

func (aws *AWS) LambdaFunctionLogsConsoleURL(functionName string) string {
	region := aws.Config.Region
	functionNameEncoded := url.QueryEscape(functionName)
	return fmt.Sprintf(
		"https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#logsV2:log-groups/log-group/%s",
		region, region, functionNameEncoded,
	)
}
