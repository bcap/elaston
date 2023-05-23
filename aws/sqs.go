package aws

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqsT "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Queue struct {
	Name       string
	URL        string
	Attributes map[string]string
}

func (aws *AWS) CreateQueue(ctx context.Context, name string) (*Queue, error) {
	_, err := aws.SQS.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: &name,
	})
	if err != nil {
		return nil, err
	}
	return aws.GetQueue(ctx, name)
}

func (aws *AWS) GetQueue(ctx context.Context, name string) (*Queue, error) {
	url, err := aws.SQS.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: &name,
	})
	if err != nil {
		return nil, err
	}
	attributes, err := aws.SQS.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       url.QueueUrl,
		AttributeNames: []sqsT.QueueAttributeName{sqsT.QueueAttributeNameAll},
	})
	if err != nil {
		return nil, err
	}
	return &Queue{Name: name, URL: *url.QueueUrl, Attributes: attributes.Attributes}, nil
}

func (aws *AWS) SendSQSJSON(ctx context.Context, queueURL string, message any) (*sqs.SendMessageOutput, error) {
	buf := bytes.Buffer{}
	if err := json.NewEncoder(&buf).Encode(message); err != nil {
		return nil, err
	}
	return aws.SendSQS(ctx, queueURL, buf.String())
}

func (aws *AWS) SendSQS(ctx context.Context, queueURL string, message string) (*sqs.SendMessageOutput, error) {
	out, err := aws.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		MessageBody: &message,
		QueueUrl:    &queueURL,
	})
	return out, err
}
