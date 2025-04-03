# This is not a Dockerfile for the server. This is a Dockerfile for the containers that will execute the code.

FROM alpine:3.21.3

RUN apk update
RUN apk add --update --no-cache python3 py3-pip
RUN apk add build-base
RUN apk add openjdk8
RUN apk add nano

CMD [ "python3", "-m", "http.server" ]