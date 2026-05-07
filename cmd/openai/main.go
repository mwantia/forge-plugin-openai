package main

import (
	"github.com/mwantia/forge-plugin-openai/plugin"
	"github.com/mwantia/forge-sdk/pkg/plugins/grpc"
)

func main() {
	grpc.Serve(plugin.NewOpenAIDriver)
}
