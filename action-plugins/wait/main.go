package main

import (
	"errors"
	"net/rpc"
	"strconv"
	"time"

	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

type Receiver struct {
	Receiver string `json:"receiver"`
}

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	waitTime := 10

	for _, param := range request.Step.Action.Params {
		if param.Key == "WaitTime" {
			waitTime, _ = strconv.Atoi(param.Value)
		}
	}

	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Wait",
				Lines: []models.Line{
					{
						Content:   `Waiting for ` + strconv.Itoa(waitTime) + ` seconds`,
						Timestamp: time.Now(),
					},
				},
			},
		},
		Status:    "paused",
		StartedAt: time.Now(),
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	executions.SetToPaused(request.Config, request.Execution, request.Platform)

	time.Sleep(time.Duration(waitTime) * time.Second)

	executions.SetToRunning(request.Config, request.Execution, request.Platform)

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Wait",
				Lines: []models.Line{
					{
						Content:   "Wait finished",
						Color:     "success",
						Timestamp: time.Now(),
					},
					{
						Content:   "Continuing to next step",
						Color:     "success",
						Timestamp: time.Now(),
					},
				},
			},
		},
		Status:     "success",
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

func (p *Plugin) CancelTask(request plugins.CancelTaskRequest) (plugins.Response, error) {
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
		Name:    "Wait",
		Type:    "action",
		Version: "1.3.1",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Wait",
			Description: "Waits for a specified amount of time",
			Plugin:      "wait",
			Icon:        "hugeicons:pause",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "WaitTime",
					Type:        "number",
					Default:     "10",
					Required:    true,
					Description: "The time to wait in seconds",
					Category:    "General",
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

func (s *PluginRPCServer) CancelTask(request plugins.CancelTaskRequest, resp *plugins.Response) error {
	result, err := s.Impl.CancelTask(request)
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
