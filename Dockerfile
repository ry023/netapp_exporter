FROM golang:1.15 AS build

workdir /opt
COPY ./go.mod /opt/go.mod
COPY ./go.sum /opt/go.sum
RUN go mod download
ENV GOOS=linux
ENV GOARCH=amd64
COPY . /opt
RUN go build -o ./netapp_exporter netapp_exporter.go

FROM ubuntu:latest

COPY --from=build /opt/netapp_exporter /opt/netapp_exporter
