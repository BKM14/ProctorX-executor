# ProctorX Go Executor

This service is responsible for running user-submitted code securely using Docker containers. It listens for tasks via Redis, then spins up isolated environments to execute them.

---

## 📦 Prerequisites

- [Go](https://golang.org/)
- [Docker](https://docs.docker.com/get-docker/)
- [Redis](https://redis.io/)

---

## ⚙️ Setup Instructions

### 1. Clone the Repository

```bash
git clone https://github.com/your-org/proctorx-executor.git
cd proctorx-executor
```

### 2. Install Go Dependencies

```bash
go mod download
go mod verify
go mod tidy
```

### 3. Configure Environment Variables

```bash
cp .example.env .env
```

Open `.env` and set the following variables:

- DOCKER_HOST - This specifies the Docker socket to connect to the Docker daemon.
- REDIS_HOST_ADDRESS - This is the address of the Redis server that the application will connect to.
- SOURCE_MOUNT - This is the directory on your host machine where the execution files will be stored.
- DESTINATION_MOUNT - This is the directory inside the Docker container where the execution files will be mounted.

### 4. Running the Executor

Make sure Docker and Redis are both running.
Run the Go application directly from your host machine:

```bash
go run .
```

This will initiate the ProctorX Go Executor service, and it will start listening for tasks via Redis.
