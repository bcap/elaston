package deploy

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type CleanupError struct {
	Deployment *Deployment
	Errors     []error
}

func (CleanupError) Error() string {
	return "failed to cleanup deployment"
}

func (d *Deployment) Clean(ctx context.Context, keepFunction bool) *CleanupError {
	log.Printf("Cleaning up deployment %s", d.ID)

	errors := []error{}
	errors = append(errors, d.deleteLambdaEventSourceMappings(ctx)...)
	if !keepFunction {
		errors = append(errors, d.deleteLambdaFunction(ctx)...)
	}
	errors = append(errors, d.deleteSQSQueue(ctx)...)
	errors = append(errors, d.deleteIAMRole(ctx)...)

	if len(errors) == 0 {
		return nil
	}
	return &CleanupError{
		Deployment: d,
		Errors:     errors,
	}
}

func (d *Deployment) deleteLambdaEventSourceMappings(ctx context.Context) []error {
	if d.Function == nil {
		return nil
	}

	mappings, err := d.aws.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: d.Function.Configuration.FunctionName,
	})
	if err != nil {
		return []error{err}
	}

	errors := []error{}
	for _, mapping := range mappings.EventSourceMappings {
		_, err := d.aws.Lambda.DeleteEventSourceMapping(ctx, &lambda.DeleteEventSourceMappingInput{
			UUID: mapping.UUID,
		})
		if err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func (d *Deployment) deleteLambdaFunction(ctx context.Context) []error {
	if d.Function == nil {
		return nil
	}

	_, err := d.aws.Lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: d.Function.Configuration.FunctionName,
	})
	if err != nil {
		return []error{err}
	}
	return nil
}

func (d *Deployment) deleteSQSQueue(ctx context.Context) []error {
	if d.Queue == nil {
		return nil
	}

	_, err := d.aws.SQS.DeleteQueue(ctx, &sqs.DeleteQueueInput{
		QueueUrl: &d.Queue.URL,
	})
	if err != nil {
		return []error{err}
	}
	return nil
}

func (d *Deployment) deleteIAMRole(ctx context.Context) []error {
	if d.Role == nil {
		return nil
	}

	errors := []error{}
	var err error

	_, err = d.aws.IAM.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
		PolicyArn: d.Role.Policy.Arn,
		RoleName:  d.Role.Role.RoleName,
	})
	if err != nil {
		errors = append(errors, err)
	}

	_, err = d.aws.IAM.DeletePolicy(ctx, &iam.DeletePolicyInput{
		PolicyArn: d.Role.Policy.Arn,
	})
	if err != nil {
		errors = append(errors, err)
	}

	_, err = d.aws.IAM.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: d.Role.Role.RoleName,
	})
	if err != nil {
		errors = append(errors, err)
	}

	return errors
}
