package main

import (
	"github.com/mwantia/forge-plugin-openai/internal/openai"
	"github.com/mwantia/forge-sdk/pkg/plugins/grpc"
)

func main() {
	grpc.Serve(openai.NewOpenAIDriver)
}
