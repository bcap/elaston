package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamT "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func (aws *AWS) GetRole(ctx context.Context, name string) (*Role, error) {
	roleOut, err := aws.IAM.GetRole(ctx, &iam.GetRoleInput{RoleName: &name})
	if err != nil {
		var noEntity *iamT.NoSuchEntityException
		if errors.As(err, &noEntity) {
			return nil, nil
		}
		return nil, err
	}

	policies, err := aws.IAM.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: &name,
	})
	if err != nil {
		return &Role{Role: roleOut.Role}, err
	}

	var policy *iamT.Policy
	for _, p := range policies.AttachedPolicies {
		if *p.PolicyName == name {
			policyOut, err := aws.IAM.GetPolicy(ctx, &iam.GetPolicyInput{
				PolicyArn: p.PolicyArn,
			})
			if err != nil {
				return &Role{Role: roleOut.Role}, err
			}
			policy = policyOut.Policy
			break
		}
	}

	return &Role{Role: roleOut.Role, Policy: policy}, nil
}

type Role struct {
	Role   *iamT.Role
	Policy *iamT.Policy
}

func (aws *AWS) CreateRole(ctx context.Context, name string, description string, assumeRolePolicyDoc string, permissionsPolicyDoc string) (*Role, error) {
	role, err := aws.IAM.CreateRole(ctx, &iam.CreateRoleInput{
		AssumeRolePolicyDocument: &assumeRolePolicyDoc,
		RoleName:                 &name,
		Description:              &description,
	})
	if err != nil {
		return nil, err
	}

	policy, err := aws.IAM.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyDocument: &permissionsPolicyDoc,
		PolicyName:     &name,
		Description:    &description,
	})
	if err != nil {
		return &Role{Role: role.Role}, err
	}

	_, err = aws.IAM.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: policy.Policy.Arn,
		RoleName:  &name,
	})

	return &Role{Role: role.Role, Policy: policy.Policy}, err
}
