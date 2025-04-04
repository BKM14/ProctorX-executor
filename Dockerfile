# This is not a Dockerfile for the server. This is a Dockerfile for the containers that will execute the code.

FROM alpine:3.21.3

RUN apk update
RUN apk add --no-cache python3 build-base py3-pip openjdk8 nano

CMD [ "python3", "-m", "http.server" ]