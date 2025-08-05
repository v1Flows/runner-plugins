package main

import (
	"context"
	"errors"
	"fmt"
	"net/rpc"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

var (
	taskCancels   = make(map[string]context.CancelFunc)
	taskCancelsMu sync.Mutex
)

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	ctx, cancel := context.WithCancel(context.Background())
	stepID := request.Step.ID.String()

	// Store cancel func
	taskCancelsMu.Lock()
	taskCancels[stepID] = cancel
	taskCancelsMu.Unlock()
	defer func() {
		taskCancelsMu.Lock()
		delete(taskCancels, stepID)
		taskCancelsMu.Unlock()
	}()

	endpointIP := ""
	endpointPort := ""
	resource := ""
	containerAction := ""
	containerID := ""
	containerRemoveVolumes := false
	containerRemoveForce := false
	containerLogsStdout := true
	containerLogsStderr := false
	imageAction := ""
	imageID := ""
	imageBuildTags := ""
	imageBuildNoCache := false
	imageBuildDockerfile := ""
	volumeAction := ""
	volumeID := ""
	networkAction := ""
	networkID := ""

	for _, param := range request.Step.Action.Params {
		if param.Key == "endpoint_ip" {
			endpointIP = param.Value
		}
		if param.Key == "endpoint_port" {
			endpointPort = param.Value
		}
		if param.Key == "resource" {
			resource = param.Value
		}
		if param.Key == "container_action" {
			containerAction = param.Value
		}
		if param.Key == "container_id" {
			containerID = param.Value
		}
		if param.Key == "container_remove_volumes" {
			containerRemoveVolumes = param.Value == "true"
		}
		if param.Key == "container_remove_force" {
			containerRemoveForce = param.Value == "true"
		}
		if param.Key == "container_logs_stdout" {
			containerLogsStdout = param.Value == "true"
		}
		if param.Key == "container_logs_stderr" {
			containerLogsStderr = param.Value == "true"
		}
		if param.Key == "image_action" {
			imageAction = param.Value
		}
		if param.Key == "image_id" {
			imageID = param.Value
		}
		if param.Key == "image_build_tags" {
			imageBuildTags = param.Value
		}
		if param.Key == "image_build_no_cache" {
			imageBuildNoCache = param.Value == "true"
		}
		if param.Key == "image_build_dockerfile" {
			imageBuildDockerfile = param.Value
		}
		if param.Key == "volume_action" {
			volumeAction = param.Value
		}
		if param.Key == "volume_id" {
			volumeID = param.Value
		}
		if param.Key == "network_action" {
			networkAction = param.Value
		}
		if param.Key == "network_id" {
			networkID = param.Value
		}
	}

	// Check for cancellation before each major step
	if ctx.Err() != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Cancel",
					Lines: []models.Line{
						{
							Content:   "Action canceled",
							Color:     "danger",
							Timestamp: time.Now(),
						},
					},
				},
			},
			Status:     "canceled",
			FinishedAt: time.Now(),
		}, request.Platform)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}

		return plugins.Response{Success: false, Canceled: true}, nil
	}

	// start the action
	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Action",
				Lines: []models.Line{
					{
						Content:   "Action started",
						Timestamp: time.Now(),
					},
					{
						Content:   "Performing action on Docker Resource: " + resource,
						Timestamp: time.Now(),
					},
					{
						Content:   "Creating Docker client connection",
						Timestamp: time.Now(),
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

	endpoint := fmt.Sprintf("tcp://%s:%s", endpointIP, endpointPort)
	client, err := docker.NewClient(endpoint)
	if err != nil {
		err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Docker Client Error",
					Lines: []models.Line{
						{
							Content:   "Failed to create Docker client",
							Color:     "danger",
							Timestamp: time.Now(),
						},
						{
							Content:   err.Error(),
							Color:     "danger",
							Timestamp: time.Now(),
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
		}, errors.New("failed to create Docker client: " + err.Error())
	}

	switch resource {
	case "container":
		err := container(client, request, containerAction, containerID, containerRemoveVolumes, containerRemoveForce, containerLogsStdout, containerLogsStderr)
		if err != nil {
			return plugins.Response{
				Success: false,
			}, err
		}
	case "image":
		_ = image(client, imageAction, imageID, imageBuildTags, imageBuildNoCache, imageBuildDockerfile)
	case "volume":
		_ = volume(client, volumeAction, volumeID)
	case "network":
		_ = network(client, networkAction, networkID)
	default:
		return plugins.Response{
			Success: false,
		}, errors.New("unknown resource type: " + resource)
	}

	// finish the action
	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Action",
				Lines: []models.Line{
					{
						Content:   "Action finished",
						Timestamp: time.Now(),
						Color:     "success",
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
	stepID := request.Step.ID.String()
	taskCancelsMu.Lock()
	cancel, ok := taskCancels[stepID]
	taskCancelsMu.Unlock()

	if !ok {
		return plugins.Response{
			Success: false,
		}, errors.New("task not found")
	}

	cancel()
	return plugins.Response{Success: true}, nil
}

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "Docker",
		Type:    "action",
		Version: "1.0.0",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Docker",
			Description: "Manage Docker containers and images",
			Plugin:      "docker",
			Icon:        "logos:docker-icon",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "endpoint_ip",
					Title:       "Docker Endpoint IP",
					Type:        "text",
					Default:     "127.0.0.1",
					Required:    false,
					Description: "Provide the Docker API endpoint ip to connect to.",
					Category:    "General",
				},
				{
					Key:         "endpoint_port",
					Title:       "Docker Endpoint Port",
					Type:        "text",
					Default:     "2375",
					Required:    false,
					Description: "Provide the Docker API endpoint port to connect to.",
					Category:    "General",
				},
				{
					Key:         "resource",
					Title:       "Resource",
					Description: "Select the type of Docker resource to manage",
					Type:        "select",
					Default:     "container",
					Required:    true,
					Category:    "General",
					Options: []models.Option{
						{
							Key:   "container",
							Value: "Container",
						},
						{
							Key:   "image",
							Value: "Image",
						},
						{
							Key:   "volume",
							Value: "Volume",
						},
						{
							Key:   "network",
							Value: "Network",
						},
					},
				},
				{
					Key:         "container_action",
					Title:       "Action",
					Description: "Select the action to perform on the Docker container",
					Type:        "select",
					Default:     "list",
					Required:    false,
					Category:    "Container",
					Options: []models.Option{
						{
							Key:   "list",
							Value: "List",
						},
						{
							Key:   "start",
							Value: "Start",
						},
						{
							Key:   "stop",
							Value: "Stop",
						},
						{
							Key:   "restart",
							Value: "Restart",
						},
						{
							Key:   "kill",
							Value: "Kill",
						},
						{
							Key:   "remove",
							Value: "Remove",
						},
						{
							Key:   "prune",
							Value: "Prune",
						},
						{
							Key:   "logs",
							Value: "Logs",
						},
					},
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "container",
					},
				},
				{
					Key:         "container_id",
					Title:       "Container ID",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Provide the ID of the Docker container to manage. Optional for list action.",
					Category:    "Container",
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "container",
					},
				},
				{
					Key:         "container_remove_volumes",
					Title:       "Remove Volumes",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Set to true to remove the Docker container's volumes.",
					Category:    "Container",
					DependsOn: models.DependsOn{
						Key:   "container_action",
						Value: "remove",
					},
				},
				{
					Key:         "container_remove_force",
					Title:       "Remove Force",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Set to true to forcefully remove the Docker container.",
					Category:    "Container",
					DependsOn: models.DependsOn{
						Key:   "container_action",
						Value: "remove",
					},
				},
				{
					Key:         "container_logs_stdout",
					Title:       "Logs Stdout",
					Type:        "boolean",
					Default:     "true",
					Required:    false,
					Description: "Set to true to stream the Docker container's stdout logs.",
					Category:    "Container",
					DependsOn: models.DependsOn{
						Key:   "container_action",
						Value: "logs",
					},
				},
				{
					Key:         "container_logs_stderr",
					Title:       "Logs Stderr",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Set to true to stream the Docker container's stderr logs.",
					Category:    "Container",
					DependsOn: models.DependsOn{
						Key:   "container_action",
						Value: "logs",
					},
				},
				{
					Key:         "image_action",
					Title:       "Action",
					Description: "Select the action to perform on the Docker image",
					Type:        "select",
					Default:     "list",
					Required:    false,
					Category:    "Image",
					Options: []models.Option{
						{
							Key:   "list",
							Value: "List",
						},
						{
							Key:   "build",
							Value: "Build",
						},
						{
							Key:   "inspect",
							Value: "Inspect",
						},
						{
							Key:   "pull",
							Value: "Pull",
						},
						{
							Key:   "push",
							Value: "Push",
						},
						{
							Key:   "remove",
							Value: "Remove",
						},
						{
							Key:   "prune",
							Value: "Prune",
						},
					},
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "image",
					},
				},
				{
					Key:         "image_id",
					Title:       "Image ID",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Provide the id of the Docker image to manage. Optional for list action.",
					Category:    "Image",
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "image",
					},
				},
				{
					Key:         "image_build_tags",
					Title:       "Build Tags",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Provide the tags to apply to the Docker image during build.",
					Category:    "Image",
					DependsOn: models.DependsOn{
						Key:   "image_action",
						Value: "build",
					},
				},
				{
					Key:         "image_build_no_cache",
					Title:       "Build No Cache",
					Type:        "boolean",
					Default:     "false",
					Required:    false,
					Description: "Set to true to build the Docker image without using cache.",
					Category:    "Image",
					DependsOn: models.DependsOn{
						Key:   "image_action",
						Value: "build",
					},
				},
				{
					Key:         "image_build_dockerfile",
					Title:       "Build Dockerfile",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Provide the Dockerfile to use for building the image.",
					Category:    "Image",
					DependsOn: models.DependsOn{
						Key:   "image_action",
						Value: "build",
					},
				},
				{
					Key:         "volume_action",
					Title:       "Volume Action",
					Description: "Select the action to perform on the Docker volume",
					Type:        "select",
					Default:     "list",
					Required:    false,
					Category:    "Volume",
					Options: []models.Option{
						{
							Key:   "list",
							Value: "List",
						},
						{
							Key:   "inspect",
							Value: "Inspect",
						},
						{
							Key:   "remove",
							Value: "Remove",
						},
						{
							Key:   "prune",
							Value: "Prune",
						},
					},
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "volume",
					},
				},
				{
					Key:         "volume_id",
					Title:       "Volume ID",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Provide the id of the Docker volume to manage. Optional for list action.",
					Category:    "Volume",
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "volume",
					},
				},
				{
					Key:         "network_action",
					Title:       "Network Action",
					Description: "Select the action to perform on the Docker network",
					Type:        "select",
					Default:     "create",
					Required:    false,
					Category:    "Network",
					Options: []models.Option{
						{
							Key:   "list",
							Value: "List",
						},
						{
							Key:   "inspect",
							Value: "Inspect",
						},
						{
							Key:   "remove",
							Value: "Remove",
						},
						{
							Key:   "prune",
							Value: "Prune",
						},
					},
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "network",
					},
				},
				{
					Key:         "network_id",
					Title:       "Network ID",
					Type:        "text",
					Default:     "",
					Required:    false,
					Description: "Provide the id of the Docker network to manage. Optional for list action.",
					Category:    "Network",
					DependsOn: models.DependsOn{
						Key:   "resource",
						Value: "network",
					},
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
