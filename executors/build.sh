#!/bin/bash

docker build -t cpp_proctorx -f ./executors/cpp.Dockerfile .
docker build -t python_proctorx -f ./executors/python.Dockerfile .
docker build -t java_proctorx -f ./executors/java.Dockerfile .