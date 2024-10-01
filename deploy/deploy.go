package deploy

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaT "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/lestrrat-go/strftime"

	"github.com/bcap/elaston/aws"
)

type Deployment struct {
	ID       string
	Function *lambda.GetFunctionOutput
	Role     *aws.Role
	Queue    *aws.Queue

	aws *aws.AWS
}

func Deploy(ctx context.Context, aws *aws.AWS, name string, executable []byte, memory int32) (Deployment, error) {
	deployment := Deployment{
		ID:  deploymentID() + "-" + name,
		aws: aws,
	}

	queueName := "elaston-queue-" + deployment.ID
	log.Printf("Deploying sqs queue %s", queueName)
	queue, err := deployQueue(ctx, aws, queueName)
	deployment.Queue = queue
	if err != nil {
		return deployment, err
	}

	roleName := "elaston-lambda-role-" + deployment.ID
	log.Printf("Deploying iam role and policy %s", roleName)
	role, err := deployRole(ctx, aws, roleName)
	deployment.Role = role
	if err != nil {
		return deployment, err
	}

	functionName := "elaston-lambda-" + deployment.ID
	log.Printf("Deploying lambda function %s", functionName)
	lambdaFn, err := deployLambdaFunction(ctx, aws, functionName, executable, memory, *role.Role.Arn, queue)
	deployment.Function = lambdaFn
	if err != nil {
		return deployment, err
	}

	log.Printf("Lambda function on AWS Console: %s", aws.LambdaFunctionConsoleURL(functionName))
	log.Printf("Lambda function logs on AWS Console: %s", aws.LambdaFunctionLogsConsoleURL(functionName))

	return deployment, nil
}

func deployRole(ctx context.Context, aws *aws.AWS, name string) (*aws.Role, error) {
	role, err := aws.GetRole(ctx, name)
	if err != nil {
		return nil, err
	}
	if role != nil {
		return role, nil
	}

	assumeRolePolicyDoc := strings.TrimSpace(`
		{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {
						"Service": "lambda.amazonaws.com"
					},
					"Action": "sts:AssumeRole"
				}
			]
		}
	`)

	permissionPolicyDoc := strings.TrimSpace(fmt.Sprintf(`
		{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Action": [
						"lambda:InvokeFunction",
						"sqs:SendMessage",
						"sqs:ReceiveMessage",
						"sqs:DeleteMessage",
						"sqs:GetQueueAttributes",
						"sqs:GetQueueUrl",
						"logs:CreateLogGroup",
						"logs:CreateLogStream",
						"logs:PutLogEvents"
					],
					"Resource": "%s"
				}
			]
		}
	`, "*"))

	return aws.CreateRole(ctx, name, "role deployed by elaston", assumeRolePolicyDoc, permissionPolicyDoc)
}

func deployQueue(ctx context.Context, aws *aws.AWS, name string) (*aws.Queue, error) {
	return aws.CreateQueue(ctx, name)
}

func deployLambdaFunction(ctx context.Context, aws *aws.AWS, name string, executable []byte, memory int32, roleARN string, queue *aws.Queue) (*lambda.GetFunctionOutput, error) {
	function, err := aws.GetLambdaFunction(ctx, name)
	if err != nil {
		return nil, err
	}

	handler := "main"

	zippedCodeBytes, err := zipExecutable(handler, executable)
	if err != nil {
		return nil, err
	}

	// https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html
	// Unforunately lambda functions only support x86_64 for golang
	arch := []lambdaT.Architecture{"x86_64"}

	queueARN := queue.Attributes["QueueArn"]

	if function == nil {
		maxWait := 10 * time.Second
		start := time.Now()
		for {
			log.Printf("Will upload %dKiB of zipped code", len(zippedCodeBytes)/1024)
			_, err = aws.Lambda.CreateFunction(ctx, &lambda.CreateFunctionInput{
				Code:          &lambdaT.FunctionCode{ZipFile: zippedCodeBytes},
				Role:          &roleARN,
				FunctionName:  &name,
				MemorySize:    &memory,
				Runtime:       "go1.x",
				Handler:       &handler,
				Architectures: arch,
				Environment: &lambdaT.Environment{
					Variables: map[string]string{
						"ELASTON_RUNNING_ON_LAMBDA": "",
						"ELASTON_SQS_QUEUE_ARN":     queueARN,
						"ELASTON_SQS_QUEUE_URL":     queue.URL,
					},
				},
			})
			if err == nil {
				break
			}
			// The special error handling below is necessary due to propagation issues inside AWS:
			// Newly created roles can take a while until they can be used by lambda functions
			var invalidParam *lambdaT.InvalidParameterValueException
			message := "The role defined for the function cannot be assumed by Lambda."
			if errors.As(err, &invalidParam) && *invalidParam.Message == message {
				if time.Since(start) > maxWait {
					return nil, err
				}
				select {
				case <-time.After(500 * time.Millisecond):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, err
		}

	} else {
		_, err := aws.Lambda.UpdateFunctionConfiguration(ctx, &lambda.UpdateFunctionConfigurationInput{
			FunctionName: &name,
			MemorySize:   &memory,
			Role:         &roleARN,
			Handler:      &handler,
		})
		if err != nil {
			return nil, err
		}

		log.Printf("Will upload %dKiB of zipped code", len(zippedCodeBytes)/1024)
		_, err = aws.Lambda.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
			FunctionName:  &name,
			ZipFile:       zippedCodeBytes,
			Architectures: arch,
		})
		if err != nil {
			return nil, err
		}
	}

	lambdaFn, err := waitLambdaDeployment(ctx, aws, name)
	if err != nil {
		return lambdaFn, err
	}

	if _, err := deployQueueTrigger(ctx, aws, name, queueARN); err != nil {
		return lambdaFn, err
	}

	return lambdaFn, nil
}

func waitLambdaDeployment(ctx context.Context, aws *aws.AWS, name string) (*lambda.GetFunctionOutput, error) {
	for {
		lambdaFn, err := aws.GetLambdaFunction(ctx, name)
		if err != nil {
			return nil, err
		}

		state := lambdaFn.Configuration.State
		if state == lambdaT.StateFailed || state == lambdaT.StateInactive {
			return lambdaFn, fmt.Errorf("lambda function in %s state", lambdaFn.Configuration.State)
		}

		switch lambdaFn.Configuration.LastUpdateStatus {
		case lambdaT.LastUpdateStatusSuccessful:
			return lambdaFn, nil
		case lambdaT.LastUpdateStatusFailed:
			return lambdaFn, errors.New("lambda function update failed")
		}

		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func deployQueueTrigger(ctx context.Context, aws *aws.AWS, name string, queueARN string) (*lambda.CreateEventSourceMappingOutput, error) {
	var batchSize int32 = 1
	return aws.Lambda.CreateEventSourceMapping(ctx, &lambda.CreateEventSourceMappingInput{
		FunctionName:   &name,
		BatchSize:      &batchSize,
		EventSourceArn: &queueARN,
	})
}

func zipExecutable(name string, data []byte) ([]byte, error) {
	buf := bytes.Buffer{}
	zipWriter := zip.NewWriter(&buf)
	zipEntryHeader := zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	zipEntryHeader.SetMode(0755)
	zipEntryWriter, err := zipWriter.CreateHeader(&zipEntryHeader)
	if err != nil {
		return nil, err
	}
	if _, err := zipEntryWriter.Write(data); err != nil {
		return nil, err
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func deploymentID() string {
	formatter, err := strftime.New("%y%m%d-%H%M%S%L", strftime.WithMilliseconds('L'))
	if err != nil {
		panic(err)
	}
	return formatter.FormatString(time.Now())
}
