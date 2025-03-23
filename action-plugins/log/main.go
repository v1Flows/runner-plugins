package main

import (
	"encoding/json"
	"errors"
	"net/rpc"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	additionalMessage := ""

	for _, param := range request.Step.Action.Params {
		if param.Key == "AdditionalMessage" {
			additionalMessage = param.Value
		}
	}

	if strings.Contains(additionalMessage, "payload.") && request.Platform == "alertflow" {
		// convert payload to string
		payloadBytes, err := json.Marshal(request.Alert.Payload)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
		payloadString := string(payloadBytes)

		additionalMessage = gjson.Get(payloadString, strings.Replace(additionalMessage, "payload.", "", 1)).String()
	}

	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Log",
				Lines: []models.Line{
					{
						Content: "Log Action started",
					},
					{
						Content: "Execution ID: " + request.Execution.ID.String(),
					},
					{
						Content: "Step ID: " + request.Step.ID.String(),
					},
					{
						Content: "Additional Message",
					},
					{
						Content: additionalMessage,
					},
					{
						Content: "Log Action finished",
					},
				},
			},
		},
		Status:     "success",
		StartedAt:  time.Now(),
		FinishedAt: time.Now(),
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	return plugins.Response{
		Success: true,
	}, nil
}

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "Log",
		Type:    "action",
		Version: "1.2.2",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Log Message",
			Description: "Logs a message in action messages",
			Plugin:      "log",
			Icon:        "solar:clipboard-list-broken",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "AdditionalMessage",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Additional message to log. To use the alert payload data, use 'payload.<key>'",
				},
			},
		},
		Endpoint: models.Endpoint{},
	}

	return plugin, nil
}

// PluginRPCServer is the RPC server for Plugin
type PluginRPCServer struct {
	Impl plugins.Plugin
}

func (s *PluginRPCServer) ExecuteTask(request plugins.ExecuteTaskRequest, resp *plugins.Response) error {
	result, err := s.Impl.ExecuteTask(request)
	*resp = result
	return err
}

func (s *PluginRPCServer) EndpointRequest(request plugins.EndpointRequest, resp *plugins.Response) error {
	result, err := s.Impl.EndpointRequest(request)
	*resp = result
	return err
}

func (s *PluginRPCServer) Info(request plugins.InfoRequest, resp *models.Plugin) error {
	result, err := s.Impl.Info(request)
	*resp = result
	return err
}

// PluginServer is the implementation of plugin.Plugin interface
type PluginServer struct {
	Impl plugins.Plugin
}

func (p *PluginServer) Server(*plugin.MuxBroker) (interface{}, error) {
	return &PluginRPCServer{Impl: p.Impl}, nil
}

func (p *PluginServer) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &plugins.PluginRPC{Client: c}, nil
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "PLUGIN_MAGIC_COOKIE",
			MagicCookieValue: "hello",
		},
		Plugins: map[string]plugin.Plugin{
			"plugin": &PluginServer{Impl: &Plugin{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
