package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AWS struct {
	Config         aws.Config
	STS            *sts.Client
	ECR            *ecr.Client
	IAM            *iam.Client
	SQS            *sqs.Client
	Lambda         *lambda.Client
	CloudWatch     *cloudwatch.Client
	CloudWatchLogs *cloudwatchlogs.Client
}

func New(profile string) *AWS {
	config := Config(profile)
	return &AWS{
		Config:         config,
		STS:            sts.NewFromConfig(config),
		ECR:            ecr.NewFromConfig(config),
		IAM:            iam.NewFromConfig(config),
		SQS:            sqs.NewFromConfig(config),
		Lambda:         lambda.NewFromConfig(config),
		CloudWatch:     cloudwatch.NewFromConfig(config),
		CloudWatchLogs: cloudwatchlogs.NewFromConfig(config),
	}
}

func Config(profile string) aws.Config {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithSharedConfigProfile(profile))
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}
	return cfg
}
