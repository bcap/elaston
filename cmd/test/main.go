package main

import (
	"context"
	"os"

	"github.com/davecgh/go-spew/spew"

	"github.com/bcap/elaston"
)

func main() {
	elaston.Run(elaston.HandlerFunc(Handler))
}

func Handler(ctx context.Context, elaston *elaston.Elaston, rawInput any) (any, error) {
	result := map[string]any{}
	set := func(name string, value any) {
		result[name] = value
		spew.Printf("HANDLER LOG | %s: %#+v\n", name, value)
	}

	input := rawInput.(map[string]any)
	if _, ok := input["submit"]; !ok {
		input["submit"] = true
		if _, err := elaston.Submit(ctx, input); err != nil {
			return nil, err
		}
	}

	set("in", input)
	set("ctx", ctx)
	set("os.environ", os.Environ())

	return result, nil
}
