package main

import (
	"errors"
	"net/rpc"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"

	"github.com/v1Flows/shared-library/pkg/models"

	"github.com/hashicorp/go-plugin"
)

// Plugin is an implementation of the Plugin interface
type Plugin struct{}

func (p *Plugin) ExecuteTask(request plugins.ExecuteTaskRequest) (plugins.Response, error) {
	from := ""
	password := ""
	to := []string{}
	smtpHost := ""
	smtpPort := 0
	message := ""

	for _, param := range request.Step.Action.Params {
		if param.Key == "From" {
			from = param.Value
		}
		if param.Key == "Password" {
			password = param.Value
		}
		if param.Key == "To" {
			to = strings.Split(param.Value, ",")
		}
		if param.Key == "SmtpHost" {
			smtpHost = param.Value
		}
		if param.Key == "SmtpPort" {
			smtpPort, _ = strconv.Atoi(param.Value)
		}
		if param.Key == "Message" {
			message = param.Value
		}
	}

	err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Mail",
				Lines: []models.Line{
					{
						Content: `Authenticate on SMTP Server: ` + smtpHost + `:` + strconv.Itoa(smtpPort),
					},
				},
			},
		},
		StartedAt: time.Now(),
		Status:    "running",
	}, request.Platform)
	if err != nil {
		return plugins.Response{
			Success: false,
		}, err
	}

	// Create authentication
	auth := smtp.PlainAuth("", from, password, smtpHost+":"+strconv.Itoa(smtpPort))

	// Send actual message
	err = smtp.SendMail(smtpHost+":"+strconv.Itoa(smtpPort), auth, from, to, []byte(message))
	if err != nil {
		err := executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
			ID: request.Step.ID,
			Messages: []models.Message{
				{
					Title: "Mail",
					Lines: []models.Line{
						{
							Content: "Failed to send email",
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
		}, nil
	}

	err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
		ID: request.Step.ID,
		Messages: []models.Message{
			{
				Title: "Mail",
				Lines: []models.Line{
					{
						Content: `Email sent to ` + strings.Join(to, ", "),
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

func (p *Plugin) EndpointRequest(request plugins.EndpointRequest) (plugins.Response, error) {
	return plugins.Response{
		Success: false,
	}, errors.New("not implemented")
}

func (p *Plugin) Info(request plugins.InfoRequest) (models.Plugin, error) {
	var plugin = models.Plugin{
		Name:    "Mail",
		Type:    "action",
		Version: "1.2.3",
		Author:  "JustNZ",
		Action: models.Action{
			Name:        "Mail",
			Description: "Send an email",
			Plugin:      "mail",
			Icon:        "solar:mailbox-linear",
			Category:    "Utility",
			Params: []models.Params{
				{
					Key:         "From",
					Type:        "text",
					Default:     "from@mail.com",
					Required:    true,
					Description: "Sender email address",
				},
				{
					Key:         "Password",
					Type:        "password",
					Default:     "***",
					Required:    false,
					Description: "Sender email password",
				},
				{
					Key:         "To",
					Type:        "text",
					Default:     "to@mail.com",
					Required:    false,
					Description: "Recipient email address. Multiple emails can be separated by comma",
				},
				{
					Key:         "SmtpHost",
					Type:        "text",
					Default:     "smtp.mail.com",
					Required:    true,
					Description: "SMTP server host",
				},
				{
					Key:         "SmtpPort",
					Type:        "number",
					Default:     "587",
					Required:    true,
					Description: "SMTP server port",
				},
				{
					Key:         "Message",
					Type:        "textarea",
					Default:     "Email message",
					Required:    true,
					Description: "Email message",
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
