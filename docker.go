package main

import (
	"fmt"

	"github.com/fsouza/go-dockerclient"
)

// ContainerForPort connects to Docker, lists all the containers
// and find the one with the port that matches the one we're
// looking for.
func ContainerForPort(socketUrl string, port int) (*docker.APIContainers, error) {
	// TODO use NewClientFromEnv()
	if len(socketUrl) < 1 {
		socketUrl = "unix:///var/run/docker.sock"
	}

	client, err := docker.NewClient(socketUrl)
	if err != nil {
		return nil, fmt.Errorf("Can't connect to Docker: %s", err)
	}

	// Make sure we list all containers, including stopped ones,
	// in the event that the container exits before we get here.
	containers, err := client.ListContainers(
		docker.ListContainersOptions{All: true},
	)
	if err != nil {
		return nil, fmt.Errorf("Can't list containers: %s", err)
	}

	port64 := int64(port)

	var foundContainer *docker.APIContainers
OUTER:
	for _, container := range containers {
		for _, p := range container.Ports {
			if p.PublicPort == port64 {
				foundContainer = &container
				break OUTER
			}
		}
	}

	if foundContainer != nil {
		return foundContainer, nil
	}

	return nil, fmt.Errorf("Unable to find container with port %d", port)
}
