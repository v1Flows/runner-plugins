package main

import (
	"errors"
	"fmt"
	"net/rpc"
	"time"

	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	kubeConfig := ""
	targetResourceType := ""
	targetResourceName := ""
	action := ""

	for _, param := range request.Step.Action.Params {
		if param.Key == "kubeConfig" {
			kubeConfig = param.Value
		}
		if param.Key == "targetResourceType" {
			targetResourceType = param.Value
		}
		if param.Key == "targetResourceName" {
			targetResourceName = param.Value
		}
		if param.Key == "action" {
			action = param.Value
		}
	}

	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "K8s",
				Lines: []models.Line{
					{
						Content: "K8s Action started",
					},
					{
						Content: "Target Resource Type: " + targetResourceType,
					},
					{
						Content: "Target Resource Name: " + targetResourceName,
					},
					{
						Content: "Action: " + action,
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

	// config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	// if err != nil {
	// 	panic(err)
	// }
	// clientset, err := kubernetes.NewForConfig(config)
	// if err != nil {
	// 	panic(err)
	// }

	fmt.Println(kubeConfig)

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
		Name:    "K8s",
		Type:    "action",
		Version: "1.0.0",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "K8s",
			Description: "Perform actions on your Kubernetes cluster",
			Plugin:      "k8s",
			Icon:        "logos:kubernetes",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "kubeConfig",
					Title:       "Kubeconfig",
					Description: "Path to the kubeconfig file",
					Type:        "text",
					Default:     "./.kube/config",
					Required:    true,
				},
				{
					Key:         "targetResourceType",
					Title:       "Target Resource Type",
					Description: "Target resource type to perform action on",
					Type:        "select",
					Options:     []string{"pod", "deployment"},
					Default:     "deployment",
					Required:    true,
				},
				{
					Key:         "targetResourceName",
					Title:       "Target Resource Name",
					Description: "Target resource name to perform action on",
					Type:        "text",
					Default:     "",
					Required:    true,
				},
				{
					Key:         "action",
					Title:       "Action",
					Description: "Action to perform",
					Type:        "select",
					Options:     []string{"restart", "scale"},
					Default:     "restart",
					Required:    true,
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
