package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {

	envErr := godotenv.Load(".env")
	if envErr != nil {
		log.Fatal("Error loading env file", envErr)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		Protocol: 2,
	})
	ctx := context.Background()

	dockerClient, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost(os.Getenv("DOCKER_HOST")))
	if err != nil {
		log.Fatal(err)
	}
	defer dockerClient.Close()
	fmt.Println("Docker Client created!")

	fmt.Println("Waiting for submissions...")
	for {
		submission := redisClient.BRPop(ctx, 0, "submissions")
		fmt.Println(submission.Val()[1])
		dockerClient.ContainerStart(ctx, "f184a78d4312", container.StartOptions{})
	}
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
