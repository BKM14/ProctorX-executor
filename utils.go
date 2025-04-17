package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/redis/go-redis/v9"
)

func publishToRedis(ctx context.Context, redisClient *redis.Client, submissionID string, output string) {
	redisClient.Publish(ctx, submissionID, output)
	fmt.Println(output)
	fmt.Printf("Successfully published on %s\n", submissionID)
}

func initDockerClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(os.Getenv("DOCKER_HOST")))
}

func initRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST_ADDRESS"),
		Password: "",
		DB:       0,
		Protocol: 2,
	})
}

func getDockerImage(lang string) string {
	images := map[string]string{
		"py":   "python_proctorx",
		"java": "java_proctorx",
		"cpp":  "cpp_proctorx",
		"c":    "cpp_proctorx",
	}

	return images[lang]
}

func getRunCommand(lang string, filename string) []string {

	var command []string

	switch lang {
	case "py":
		command = []string{"sh", "-c", fmt.Sprintf("python3 /executions/%s", filename)}

	case "java":
		className := strings.Split(filename, ".")[0]
		command = []string{"sh", "-c", fmt.Sprintf("javac /executions/%s && java -cp /executions %s", filename, className)}

	case "cpp":
		outFile := "/executions/" + strings.Split(filename, ".")[0]
		command = []string{"sh", "-c", fmt.Sprintf("g++ /executions/%s -o %s && %s", filename, outFile, outFile)}

	case "c":
		outFile := "/executions/" + strings.Split(filename, ".")[0]
		command = []string{"sh", "-c", fmt.Sprintf("gcc /executions/%s -o %s && %s", filename, outFile, outFile)}

	default:
		log.Printf("Unsupported language: %s", lang)
		return nil
	}

	return command
}

func removeUserFiles(filename string, task Task) {
	os.Remove("executions/" + filename)
	if task.Lang == "java" {
		os.Remove("executions/" + strings.Split(filename, ".")[0] + ".class")
	} else if task.Lang == "cpp" || task.Lang == "c" {
		os.Remove("executions/" + strings.Split(filename, ".")[0])
	}
}

func cleanupContainer(ctx context.Context, cli *client.Client, containerID string) {
	if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		log.Printf("Error removing container %s: %v", containerID, err)
	}
}
