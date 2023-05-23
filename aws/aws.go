package aws

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamT "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaT "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AWS struct {
	Config aws.Config
	STS    *sts.Client
	ECR    *ecr.Client
	IAM    *iam.Client
	Lambda *lambda.Client
}

func New(profile string) *AWS {
	config := Config(profile)
	return &AWS{
		Config: config,
		STS:    sts.NewFromConfig(config),
		ECR:    ecr.NewFromConfig(config),
		IAM:    iam.NewFromConfig(config),
		Lambda: lambda.NewFromConfig(config),
	}
}

func (aws *AWS) DeployLambdaFunction(ctx context.Context, name string, executable []byte, memory int32) error {
	roleName := "elaston-lambda-" + name
	role, err := aws.GetRole(ctx, roleName)
	if err != nil {
		return err
	}
	if role == nil {
		policyDoc := strings.TrimSpace(`
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
			}`)
		role, err = aws.CreateRole(ctx, roleName, "role deployed by elaston", policyDoc)
		if err != nil {
			return err
		}
	}

	handler := "main"

	buf := bytes.Buffer{}
	zipWriter := zip.NewWriter(&buf)
	zipEntryHeader := zip.FileHeader{
		Name:   handler,
		Method: zip.Deflate,
	}
	zipEntryHeader.SetMode(0755)
	zipEntryWriter, err := zipWriter.CreateHeader(&zipEntryHeader)
	if err != nil {
		return err
	}
	if _, err := zipEntryWriter.Write(executable); err != nil {
		return err
	}
	if err := zipWriter.Close(); err != nil {
		return err
	}
	zippedCodeBytes := buf.Bytes()

	if err := os.WriteFile("lambda.zip", zippedCodeBytes, 0644); err != nil {
		return err
	}

	function, err := aws.GetLambdaFunction(ctx, name)
	if err != nil {
		return nil
	}

	// https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html
	// Unforunately lambda functions only support x86_64 for golang
	arch := []lambdaT.Architecture{"x86_64"}

	if function == nil {
		log.Printf("lambda.CreateFunction")
		_, err = aws.Lambda.CreateFunction(ctx, &lambda.CreateFunctionInput{
			Code:          &lambdaT.FunctionCode{ZipFile: zippedCodeBytes},
			Role:          role.Arn,
			FunctionName:  &name,
			MemorySize:    &memory,
			Runtime:       "go1.x",
			Handler:       &handler,
			Architectures: arch,
		})
		if err != nil {
			return err
		}

	} else {
		log.Printf("lambda.UpdateFunctionConfiguration")
		_, err := aws.Lambda.UpdateFunctionConfiguration(ctx, &lambda.UpdateFunctionConfigurationInput{
			FunctionName: &name,
			MemorySize:   &memory,
			Role:         role.Arn,
			Handler:      &handler,
		})
		if err != nil {
			return err
		}

		log.Printf("lambda.UpdateFunctionCode")
		_, err = aws.Lambda.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
			FunctionName:  &name,
			ZipFile:       zippedCodeBytes,
			Architectures: arch,
		})
		if err != nil {
			return err
		}
	}

	for {
		f, err := aws.GetLambdaFunction(ctx, name)
		if err != nil {
			return err
		}
		if f.Configuration.LastUpdateStatus != "InProgress" {
			break
		}
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

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
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
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

func (aws *AWS) GetRole(ctx context.Context, name string) (*iamT.Role, error) {
	out, err := aws.IAM.GetRole(ctx, &iam.GetRoleInput{RoleName: &name})
	if err != nil {
		var noEntity *iamT.NoSuchEntityException
		if errors.As(err, &noEntity) {
			return nil, nil
		}
		return nil, err
	}
	return out.Role, nil
}

func (aws *AWS) CreateRole(ctx context.Context, name string, description string, policyDocument string) (*iamT.Role, error) {
	out, err := aws.IAM.CreateRole(ctx, &iam.CreateRoleInput{
		AssumeRolePolicyDocument: &policyDocument,
		RoleName:                 &name,
		Description:              &description,
	})
	if err != nil {
		return nil, err
	}
	return out.Role, nil
}

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

func Config(profile string) aws.Config {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithSharedConfigProfile(profile))
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}
	return cfg
}
