package main

import (
	"fmt"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/v1Flows/runner/pkg/executions"
	"github.com/v1Flows/runner/pkg/plugins"
	"github.com/v1Flows/shared-library/pkg/models"
)

func container(client *docker.Client, request plugins.ExecuteTaskRequest, action string, id string, removeVolumes bool, removeForce bool, logsStdout bool, logsStderr bool) (err error) {
	switch action {
	case "list":
		containers, err := client.ListContainers(docker.ListContainersOptions{})
		if err != nil {
			executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Docker Client Error",
						Lines: []models.Line{
							{
								Content:   "Failed to list containers: " + err.Error(),
								Color:     "danger",
								Timestamp: time.Now(),
							},
						},
					},
				},
				Status:     "error",
				FinishedAt: time.Now(),
			}, request.Platform)

			return err
		}

		for _, ctr := range containers {
			container := fmt.Sprintf("%s (%s) | Status: %s", ctr.Names[0], ctr.ID, ctr.Status)
			if len(ctr.Names) > 1 {
				container += fmt.Sprintf(" (%d aliases)", len(ctr.Names)-1)
			}

			err = executions.UpdateStep(request.Config, request.Execution.ID.String(), models.ExecutionSteps{
				ID: request.Step.ID,
				Messages: []models.Message{
					{
						Title: "Container List",
						Lines: []models.Line{
							{
								Content:   container,
								Timestamp: time.Now(),
							},
						},
					},
				},
			}, request.Platform)
			if err != nil {
				return err
			}
		}
		return nil
	case "start":
		return err
	case "stop":
		return err
	case "restart":
		return err
	case "kill":
		return err
	case "remove":
		return err
	case "prune":
		return err
	case "logs":
		return err
	default:
		return err
	}
}

func image(client *docker.Client, action string, id string, buildTags string, buildNoCache bool, buildDockerfile string) (err bool) {
	return true
}

func volume(client *docker.Client, action string, id string) (err bool) {
	return true
}

func network(client *docker.Client, action string, id string) (err bool) {
	return true
}
