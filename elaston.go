package elaston

import (
	"context"
	"encoding/json"
	"unsafe"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/bcap/elaston/aws"
)

type Handler interface {
	Handle(context.Context, *Elaston, any) (any, error)
}

type HandlerFunc func(context.Context, *Elaston, any) (any, error)

func (f HandlerFunc) Handle(ctx context.Context, elaston *Elaston, in any) (any, error) {
	return f(ctx, elaston, in)
}

type elaston struct {
	aws          *aws.AWS
	functionName string
	sqsQueueURL  string
}

type Elaston struct {
	elaston
}

type Option = func(*elaston)

func New(aws *aws.AWS, functionName string, sqsQueueURL string, options ...Option) *Elaston {
	elaston := elaston{
		functionName: functionName,
		sqsQueueURL:  sqsQueueURL,
		aws:          aws,
	}

	for _, opt := range options {
		opt(&elaston)
	}

	return &Elaston{
		elaston: elaston,
	}
}

func (e *Elaston) Call(ctx context.Context, in any) (any, error) {
	var out any
	payload, err := json.Marshal(in)
	if err != nil {
		return out, err
	}

	invocation, err := e.aws.Lambda.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: &e.functionName,
		Payload:      payload,
	})
	if err != nil {
		return out, err
	}

	if err := json.Unmarshal(invocation.Payload, &out); err != nil {
		return out, err
	}

	return out, nil
}

func (e *Elaston) Submit(ctx context.Context, in any) (string, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	// Trick used by strings.Builder.String to convert bytes to string without copying data
	payloadString := unsafe.String(unsafe.SliceData(payload), len(payload))

	sendOut, err := e.aws.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		MessageBody: &payloadString,
		QueueUrl:    &e.sqsQueueURL,
	})

	if err != nil {
		return "", err
	}

	if sendOut.SequenceNumber != nil && *sendOut.SequenceNumber != "" {
		return *sendOut.MessageId + "," + *sendOut.SequenceNumber, nil
	}

	return *sendOut.MessageId, nil
}
