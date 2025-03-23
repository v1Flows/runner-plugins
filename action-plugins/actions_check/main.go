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
	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Actions Checks",
				Lines: []models.Line{
					{
						Content: "Checking for actions in flow",
					},
				},
			},
		},
		Status:    "running",
		StartedAt: time.Now(),
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	// check if flow got any action
	if len(request.Flow.Actions) > 0 {
		count := 0
		for _, action := range request.Flow.Actions {
			if action.Active {
				count++
			}
		}

		if count == 0 {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Actions Checks",
						Lines: []models.Line{
							{
								Content: "Flow has no Actions defined. Cancel execution",
								Color:   "danger",
							},
						},
					},
				},
				Status:     "canceled",
				CanceledBy: "Flow Action Check",
				CanceledAt: time.Now(),
				FinishedAt: time.Now(),
			}, request.Platform)
			if err != nil {
				return plugins.Response{
					Success: false,
				}, err
			}
			return plugins.Response{
				Data: map[string]interface{}{
					"status": "canceled",
				},
				Success: false,
			}, nil
		} else {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Actions Checks",
						Lines: []models.Line{
							{
								Content: "Found " + strconv.Itoa(count) + " actions in flow",
								Color:   "success",
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
	} else {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Actions Checks",
					Lines: []models.Line{
						{
							Content: "Flow has no Actions defined",
							Color:   "danger",
						},
						{
							Content: "Please add actions to the flow",
						},
						{
							Content: "Cancel execution",
							Color:   "danger",
						},
					},
				},
			},
			Status:     "canceled",
			CanceledBy: "Flow Action Check",
			CanceledAt: time.Now(),
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
		return plugins.Response{
			Data: map[string]interface{}{
				"status": "canceled",
			},
			Success: false,
		}, nil
	}
}

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "Actions Check",
		Type:    "action",
		Version: "1.2.3",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Actions Check",
			Description: "Check for actions in flow",
			Plugin:      "actions_check",
			Icon:        "solar:bolt-linear",
			Category:    "Flow",
			Params:      nil,
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
