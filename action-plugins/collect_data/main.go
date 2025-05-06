package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/rpc"
	"strconv"
	"time"

	"github.com/v1Flows/runner/pkg/alerts"
	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/flows"
	"github.com/v1Flows/runner/pkg/plugins"

	af_models "github.com/v1Flows/alertFlow/services/backend/pkg/models"
	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

type Receiver struct {
	Receiver string `json:"receiver"`
}

type IncomingFlow struct {
	Flow models.Flows `json:"flow"`
}

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	flowID := ""
	alertID := ""
	logData := false

	if val, ok := request.Args["FlowID"]; ok {
		flowID = val
	}

	if val, ok := request.Args["AlertID"]; ok {
		alertID = val
	}

	if val, ok := request.Args["LogData"]; ok {
		logData, _ = strconv.ParseBool(val)
	}

	if request.Step.Action.Params != nil && flowID == "" && alertID == "" {
		for _, param := range request.Step.Action.Params {
			if param.Key == "LogData" {
				logData, _ = strconv.ParseBool(param.Value)
			}
			if param.Key == "FlowID" {
				flowID = param.Value
			}
			if param.Key == "AlertID" {
				alertID = param.Value
			}
		}
	}

	if flowID == "" || (request.Platform == "alertflow" && alertID == "") {
		_ = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Collecting Data",
					Lines: []models.Line{
						{
							Content: "FlowID and AlertID are required",
							Color:   "danger",
						},
					},
				},
			},
			Status:     "error",
			FinishedAt: time.Now(),
		}, request.Platform)

		return plugins.Response{
			Success: false,
		}, errors.New("flowid and alertid are required")
	}

	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Collecting Data",
				Lines: []models.Line{
					{
						Content: "Collecting data from " + request.Platform,
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

	// Get Flow Data
	flowBytes, err := flows.GetFlowData(request.Config, flowID, request.Platform)
	if err != nil && flowBytes == nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Collecting Data",
					Lines: []models.Line{
						{
							Content: "Failed to get Flow Data",
							Color:   "danger",
						},
						{
							Content: err.Error(),
							Color:   "danger",
						},
					},
				},
			},
			Status:     "error",
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		return plugins.Response{
			Success: false,
		}, err
	}

	flow := IncomingFlow{}
	err = json.Unmarshal(flowBytes, &flow)
	if err != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Collecting Data",
					Lines: []models.Line{
						{
							Content: "Failed to unmarshal Flow Data",
							Color:   "danger",
						},
						{
							Content: err.Error(),
							Color:   "danger",
						},
					},
				},
			},
			Status:     "error",
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Collecting Data",
				Lines: []models.Line{
					{
						Content: "Flow Data collected",
					},
				},
			},
		},
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	var alert af_models.Alerts
	if request.Platform == "alertflow" && alertID != "" {
		// Get Alert Data
		alert, err = alerts.GetData(request.Config, alertID)
		if err != nil {
			err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Collecting Data",
						Lines: []models.Line{
							{
								Content: "Failed to get Alert Data",
								Color:   "danger",
							},
							{
								Content: err.Error(),
								Color:   "danger",
							},
						},
					},
				},
				Status:     "error",
				FinishedAt: time.Now(),
			}, request.Platform)
			if err != nil {
				return plugins.Response{
					Success: false,
				}, err
			}

			return plugins.Response{
				Success: false,
			}, err
		}

		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Collecting Data",
					Lines: []models.Line{
						{
							Content: "Alert Data collected",
							Color:   "success",
						},
					},
				},
			},
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
	}

	finalMessages := []models.Line{}

	if logData {
		finalMessages = append(finalMessages, models.Line{
			Content: "Data collection completed",
			Color:   "success",
		})
		finalMessages = append(finalMessages, models.Line{
			Content: "Flow Data:",
		})
		finalMessages = append(finalMessages, models.Line{
			Content: fmt.Sprintf("%v", flow.Flow),
		})
		if request.Platform == "alertflow" && alertID != "" {
			finalMessages = append(finalMessages, models.Line{
				Content: "Alert Data:",
			})
			finalMessages = append(finalMessages, models.Line{
				Content: fmt.Sprintf("%v", alert),
			})
		}
	} else {
		finalMessages = append(finalMessages, models.Line{
			Content: "Data collection completed",
			Color:   "success",
		})
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Collecting Data",
				Lines: finalMessages,
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
		Flow:      &flow.Flow,
		FlowBytes: flowBytes,
		Alert:     &alert,
		Success:   true,
	}, nil
}

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "Collect Data",
		Type:    "action",
		Version: "1.2.6",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Collect Data",
			Description: "Collects Flow and Alert data from AlertFlow or exFlow",
			Plugin:      "collect_data",
			Icon:        "hugeicons:package-receive",
			Category:    "Data",
			Params: []models.Params{
				{
					Key:         "LogData",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Show collected data in the output messages",
					Category:    "General",
				},
				{
					Key:         "FlowID",
					Type:        "text",
					Default:     "00000000-0000-0000-0000-00000000",
					Required:    true,
					Description: "The Flow ID to collect data from",
					Category:    "General",
				},
				{
					Key:         "AlertID",
					Type:        "text",
					Default:     "00000000-0000-0000-0000-00000000",
					Required:    false,
					Description: "The Alert ID to collect data from. Required for AlertFlow platform",
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
