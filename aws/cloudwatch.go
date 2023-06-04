package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlT "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func (aws *AWS) ListLogStreams(ctx context.Context, logGroup string) ([]cwlT.LogStream, error) {
	out, err := aws.CloudWatchLogs.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: &logGroup,
	})
	if err != nil {
		return nil, err
	}
	return out.LogStreams, nil
}
