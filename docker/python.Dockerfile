# This is not a Dockerfile for the server. This is a Dockerfile for the containers that will execute the code.

FROM alpine:3.21.3

RUN apk add --no-cache python3 py3-pip

CMD ["sleep", "infinity"]