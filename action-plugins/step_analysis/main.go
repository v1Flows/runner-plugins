package main

import (
	"errors"
	"net/rpc"

	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "Step Analysis",
		Type:    "action",
		Version: "1.0.0",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Step Analysis",
			Description: "Analyse the previous steps and their messages",
			Plugin:      "step_analysis",
			Icon:        "hugeicons:package-search",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "stepID",
					Title:       "Step ID",
					Description: "ID of the step to analyse",
					Type:        "text",
					Default:     "",
					Required:    true,
					Category:    "General",
				},
				{
					Key:         "regex",
					Title:       "Regex",
					Description: "Regex to match the messages",
					Type:        "boolean",
					Default:     "false",
					Required:    true,
					Category:    "Check",
				},
				{
					Key:         "regexPattern",
					Title:       "Regex Pattern",
					Description: "Regex pattern to match the messages",
					Type:        "text",
					Default:     "",
					Required:    false,
					Category:    "Check",
				},
				{
					Key:         "string",
					Title:       "String",
					Description: "String to match the messages. If regex is true, this will be ignored",
					Type:        "text",
					Default:     "",
					Required:    false,
					Category:    "Check",
				},
				{
					Key:         "endsWith",
					Title:       "Ends With",
					Description: "If the condition is true, in which status should the step end",
					Type:        "select",
					Default:     "success",
					Required:    true,
					Category:    "Ending",
					Options: []models.Option{
						{
							Key:   "success",
							Value: "Success",
						},
						{
							Key:   "error",
							Value: "Error",
						},
						{
							Key:   "warning",
							Value: "Warning",
						},
						{
							Key:   "noPatternMatch",
							Value: "No Pattern Match",
						},
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
