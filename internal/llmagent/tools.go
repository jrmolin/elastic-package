// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package llmagent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/elastic/elastic-package/internal/configuration/locations"
)

// The embedded example_readme is an example of a high-quality integration readme, following the static template archetype,
// which will help the LLM follow an example.
//

var (
	//go:embed _static/example_readme.md
	exampleReadmeContent string

	// should these be done per-tool?
	ctx       = context.Background()
	transport mcp.Transport
)

type MCPServer struct {
	Command *string            `json:"command"`
	Args    []string           `json:"args"`
	Env     *map[string]string `json:"env"`
	Url     *string            `json:"url"`
	Headers *map[string]string `json:"headers"`

	session *mcp.ClientSession
	Tools   []Tool
}

type MCPJson struct {
	InitialPrompt  *string              `json:"initialPromptFile"`
	RevisionPrompt *string              `json:"revisionPromptFile"`
	Servers        map[string]MCPServer `json:"mcpServers"`
}

// need an MCP struct that holds an array of close functions and also an array of tools
func (s *MCPServer) Connect() error {

	ctx := context.Background()
	var transport mcp.Transport

	transport = &mcp.StreamableClientTransport{Endpoint: *(s.Url)}

	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	fmt.Printf("attempt to connect to %s\n", *(s.Url))

	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return err
	}

	s.session = cs

	// unmarshal the mcp file into a map of new servers
	// {
	//   "mcpServers": {
	//     "name": {
	//       "url": "http://localhost:8080",
	//       "headers": {
	//         "Authorization": "Bearer YOUR_GITHUB_PAT"
	//       }
	//     }
	//   }
	// }

	// type ToolHandler func(ctx context.Context, arguments string) (*ToolResult, error)
	// need to iterate over the tools and then return those
	if (*s.session).InitializeResult().Capabilities.Tools != nil {
		for feat, err := range (*s.session).Tools(ctx, nil) {
			if err != nil {
				log.Fatal(err)
			}

			// pull out the properties and required
			//

			required := feat.InputSchema.(map[string]any)["required"]
			if required == nil {
				required = []string{}
			}

			properties := feat.InputSchema.(map[string]interface{})["properties"]

			s.Tools = append(s.Tools, Tool{
				Name:        feat.Name,
				Description: feat.Description,
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": properties,
					"required":   required,
				},
				Handler: func(ctx context.Context, arguments string) (*ToolResult, error) {
					myCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()
					res, err := s.session.CallTool(myCtx, &mcp.CallToolParams{Name: feat.Name, Arguments: json.RawMessage(arguments)})

					if err != nil {
						fmt.Printf("failed to call tool with error %v", err)
						return nil, err
					}
					data, err := json.Marshal(res)
					if err != nil {
						return nil, err
					}
					return &ToolResult{Content: string(data)}, nil
				},
			})
		}
	}

	return nil
}

// PackageTools creates the tools available to the LLM for package operations.
// These tools do not allow access to `docs/`, to prevent the LLM from confusing the generated and non-generated README versions.
func MCPTools() *MCPJson {
	// what MCP servers can we connect to?
	// the handler will have a connection to the endpoint already established
	// we will create an mcp.StreamableClientTransport{Endpoint: url} for each endpoint
	// we will then list all the tools and read the description and arguments
	// look in the elastic-package config dir for mcp.json
	// LocationManager MCPJson() --> path/to/.elastic-package/mcp.json file
	lm, err := locations.NewLocationManager()
	if err != nil {
		return nil
	}

	// if the file doesn't exist, just bail
	mcpFile, err := os.Open(lm.MCPJson())
	if err != nil {
		return nil
	}
	defer mcpFile.Close()

	byteValue, err := ioutil.ReadAll(mcpFile)
	if err != nil {
		return nil
	}

	var mcpJson MCPJson
	json.Unmarshal(byteValue, &mcpJson)

	// handle the url thing only for now
	for key, value := range mcpJson.Servers {
		if value.Url != nil {
			err = value.Connect()
			mcpJson.Servers[key] = value
		}

	}

	return &mcpJson
}

func PackageTools(packageRoot string) []Tool {
	return []Tool{
		{
			Name:        "get_package_root",
			Description: "Query the system for the current package root",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
			Handler: getPackageRootHandler(packageRoot),
		},
	}
}

// getPackageRootHandler returns a handler for the get_package_root tool
func getPackageRootHandler(packageRoot string) ToolHandler {
	cleanRoot := filepath.Clean(packageRoot)
	return func(ctx context.Context, arguments string) (*ToolResult, error) {
		// Get the embedded example content
		return &ToolResult{Content: cleanRoot}, nil
	}
}
