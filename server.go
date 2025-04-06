package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type Task struct {
	Lang string `json:"lang"`
	Code string `json:"code"`
	ID   string `json:"id"`
}

func main() {

	envErr := godotenv.Load(".env")
	if envErr != nil {
		log.Fatal("Error loading env file", envErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisClient := initRedisClient()
	dockerClient, err := initDockerClient()

	if err != nil {
		log.Fatal(err)
	}
	defer dockerClient.Close()

	fmt.Println("Redis Client created!")
	fmt.Println("Docker Client created!")

	processSubmission(ctx, redisClient, dockerClient)
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

func processSubmission(ctx context.Context, redisClient *redis.Client, dockerClient *client.Client) {
	containerID := "2f5200a742a2"
	for {
		submission := redisClient.BRPop(ctx, 0, "submissions")
		result, err := submission.Result()
		if err != nil {
			log.Printf("Error retrieving result from BRPop: %v", err)
			return
		}

		if len(result) < 2 {
			log.Println("Unexpected Redis BRPop response, skipping...")
			return
		}

		jsonData := result[1]

		var task Task
		err = json.Unmarshal([]byte(jsonData), &task)
		if err != nil {
			log.Printf("Error unmarshaling JSON data: %v", err)
			return
		}

		fmt.Printf("Processing task - Language: %s\nCode: %s\n", task.Lang, task.Code)

		executeTaskInContainer(ctx, dockerClient, task, redisClient, containerID)
	}
}

func executeTaskInContainer(ctx context.Context, dockerClient *client.Client, task Task, redisClient *redis.Client, containerID string) {

	err := dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		log.Fatalf("Error starting container %s: %v", containerID, err)
		return
	}

	exec, err := dockerClient.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"python3", "-c", task.Code},
		AttachStdout: true,
		AttachStderr: true,
	})

	if err != nil {
		log.Fatalf("Error creating exec: %s", err)
	}

	execID := exec.ID

	response, err := dockerClient.ContainerExecAttach(ctx, execID, container.ExecAttachOptions{
		Tty: false,
	})
	if err != nil {
		log.Fatalf("Error attaching exec to container: %s", err)
	}
	defer response.Close()

	err = dockerClient.ContainerExecStart(ctx, execID, container.ExecStartOptions{})
	if err != nil {
		log.Fatalf("Error starting container exec: %s", err)
	}

	readLogs(response, ctx, redisClient, task.ID)
}

func readLogs(response types.HijackedResponse, ctx context.Context, redisClient *redis.Client, submissionID string) {
	output, err := io.ReadAll(response.Reader)
	if err != nil {
		log.Printf("Error reading output: %v", err)
	} else {
		fmt.Println(string(output))
	}

	fmt.Println("Execution completed successfully!")

	publishToRedis(ctx, redisClient, submissionID, string(output))
}

func publishToRedis(ctx context.Context, redisClient *redis.Client, submissionID string, output string) {
	redisClient.Publish(ctx, submissionID, output)
	fmt.Println(output)
	fmt.Printf("Successfully published on %s\n", submissionID)
}

func server() {
	fmt.Println("Starting server on port 8080")
	http.HandleFunc("/listContainers", func(w http.ResponseWriter, r *http.Request) {

		envErr := godotenv.Load(".env")
		if envErr != nil {
			log.Fatal("Error loading env file", envErr)
		}

		cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(os.Getenv("DOCKER_HOST")))
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()
		fmt.Println("Client created!")

		containers, err := cli.ContainerList(context.Background(), container.ListOptions{})
		if err != nil {
			panic(err)
		}

		for _, container := range containers {
			fmt.Fprintf(w, "Container ID: %s, Image: %s, Names: %v\n", container.ID, container.Image, container.Names)
		}
		defer cli.Close()
	})

	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Query Parameters: ", r.URL.Query())
		fmt.Fprintf(w, "Hello, World!")
	})

	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}
