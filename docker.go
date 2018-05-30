package main

import (
	"fmt"

	"github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// ContainerForPort connects to Docker, lists all the containers
// and find the one with the port that matches the one we're
// looking for.
func ContainerForPort(socketUrl string, port int) (*docker.APIContainers, error) {
	var client *docker.Client
	var err error

	if len(socketUrl) > 0 {
		client, err = docker.NewClient(socketUrl)
	} else {
		client, err = docker.NewClientFromEnv()
	}

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

// EnvAndSvcName returns the environment and service names from
// Docker labels if present.
func EnvAndSvcName(port int) (envName string, svcName string) {
	container, err := ContainerForPort("", port)
	if err != nil {
		log.Fatalf("Unable to find container for %d! (%s)", port, err)
	}

	envName = container.Labels[EnvironmentNameLabel]
	svcName = container.Labels[ServiceNameLabel]

	return envName, svcName
}
