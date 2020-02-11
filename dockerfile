FROM golang:1.12 as builder
WORKDIR /cni-terway
COPY . .
ENV GO111MODULE on
ENV GOPROXY https://goproxy.cn
RUN 

FROM generals/alpine