#!/bin/bash
protoc -I . ./protoc/positionService.proto --go_out=:.
protoc -I . ./protoc/positionService.proto --go-grpc_out=:.

