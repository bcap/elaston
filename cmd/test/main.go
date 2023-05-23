package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/bcap/elaston/aws"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/davecgh/go-spew/spew"
)

func main() {
	if _, ok := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME"); ok {
		lambda.Start(LambdaHandler)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	aws := aws.New("bcap")

	name := "elaston-test"

	err := aws.DeployLambdaFunction(ctx, name, programBytes(), 128)
	panicOnErr(err)

	stream, err := aws.InvokeLambdaFunction(ctx, name, map[string]any{"a": 1})
	panicOnErr(err)

	for ev := range stream.Reader.Events() {
		spew.Dump(ev)
	}
}

func LambdaHandler(ctx context.Context, in any) (any, error) {
	log.Printf("ctx: %s", spew.Sdump(ctx))
	log.Printf("in:  %s", spew.Sdump(in))
	log.Printf("os.environ: %s", spew.Sdump(os.Environ()))
	return in, nil
}

func programBytes() []byte {
	f, err := os.OpenFile(os.Args[0], os.O_RDONLY, 0)
	panicOnErr(err)
	data, err := io.ReadAll(f)
	panicOnErr(err)
	return data
}

func panicOnErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
