package main

import (
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

type DockerSettings struct {
	ServiceName     string
	EnvironmentName string
	ProxyMode       string
}

type DiscoveryClient interface {
	ContainerFieldsForPort(port int) (*DockerSettings, error)
}

type DockerClient struct{}

// getClient connects and returns a Docker client
func (d *DockerClient) getClient(socketUrl string) (client *docker.Client, err error) {
	if len(socketUrl) > 0 {
		client, err = docker.NewClient(socketUrl)
	} else {
		client, err = docker.NewClientFromEnv()
	}

	if err != nil {
		return nil, fmt.Errorf("Can't connect to Docker: %s", err)
	}

	return client, nil
}

// ContainerForPort connects to Docker, lists all the containers
// and find the one with the port that matches the one we're
// looking for.
func (d *DockerClient) ContainerForPort(socketUrl string, port int) (*docker.APIContainers, error) {
	var err error

	client, err := d.getClient(socketUrl)
	if err != nil {
		return nil, err
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

// ContainerFieldsForPort returns a selection of metadata lifted from
// Docker labels if present.
func (d *DockerClient) ContainerFieldsForPort(port int) (*DockerSettings, error) {
	container, err := d.ContainerForPort("", port)
	if err != nil {
		return nil, fmt.Errorf("Unable to find container for %d! (%s)", port, err)
	}

	proxyMode := container.Labels[ProxyModeLabel]
	if len(proxyMode) < 1 {
		proxyMode = "http"
	}

	return &DockerSettings{
		EnvironmentName: container.Labels[EnvironmentNameLabel],
		ServiceName:     container.Labels[ServiceNameLabel],
		ProxyMode:       strings.ToLower(proxyMode),
	}, nil
}
