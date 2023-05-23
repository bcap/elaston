package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func (aws *AWS) Account(ctx context.Context) (string, error) {
	identity, err := aws.Identity(ctx)
	if err != nil {
		return "", err
	}
	return *identity.Account, nil
}

func (aws *AWS) Identity(ctx context.Context) (*sts.GetCallerIdentityOutput, error) {
	stsClient := sts.NewFromConfig(aws.Config)
	return stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
}
