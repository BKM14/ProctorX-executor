package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"bytes"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
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
	containerID := "9b8169c7a399"
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

		filename := uuid.NewString() + "." + task.Lang
		filename = strings.Replace(filename, "-", "_", -1)

		if task.Lang == "java" {
			filename = "Main_" + filename
		}

		file, err := os.Create("executions/" + filename)
		if err != nil {
			panic(err)
		}

		var finalCode string

		if task.Lang == "java" {
			finalCode = strings.Replace(task.Code, "public class Main", "public class "+strings.Split(filename, ".")[0], -1)
		} else {
			finalCode = task.Code
		}

		_, err = file.WriteString(finalCode)
		if err != nil {
			panic(err)
		}

		file.Close()

		fmt.Printf("Processing task - Language: %s\nCode: %s\n", task.Lang, task.Code)

		var command []string

		switch task.Lang {
		case "py":
			command = []string{"python3", "/executions/" + filename}

		case "java":
			className := strings.Split(filename, ".")[0]
			command = []string{"sh", "-c", "javac /executions/" + filename + " && java -cp /executions " + className}

		case "cpp":
			outFile := "/executions/" + strings.Split(filename, ".")[0]
			command = []string{"sh", "-c", "g++ /executions/" + filename + " -o " + outFile + " && " + outFile}

		case "c":
			outFile := "/executions/" + strings.Split(filename, ".")[0]
			command = []string{"sh", "-c", "gcc /executions/" + filename + " -o " + outFile + " && " + outFile}

		default:
			log.Printf("Unsupported language: %s", task.Lang)
			return
		}

		executeTaskInContainer(ctx, dockerClient, command, redisClient, containerID, task, filename)
	}
}

func executeTaskInContainer(ctx context.Context, dockerClient *client.Client, command []string, redisClient *redis.Client, containerID string, task Task, filename string) {

	err := dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		log.Fatalf("Error starting container %s: %v", containerID, err)
		return
	}

	exec, err := dockerClient.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          command,
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

	os.Remove("executions/" + filename)
	if task.Lang == "java" {
		os.Remove("executions/" + strings.Split(filename, ".")[0] + ".class")
	} else if task.Lang == "cpp" || task.Lang == "c" {
		os.Remove("executions/" + strings.Split(filename, ".")[0])
	}
}

func readLogs(response types.HijackedResponse, ctx context.Context, redisClient *redis.Client, submissionID string) {

	var stdoutBuf, stderrBuf bytes.Buffer

	_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, response.Reader)
	if err != nil {
		log.Printf("Error copying output: %v", err)
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if stderr != "" {
		log.Printf("Stderr: %s", stderr)
		publishToRedis(ctx, redisClient, submissionID, stderr)
		return
	}

	fmt.Printf("Stdout: %s\n", stdout)
	fmt.Println("Execution completed successfully!")

	publishToRedis(ctx, redisClient, submissionID, stdout)
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
