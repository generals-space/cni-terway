## docker build --no-cache=true -f dockerfile -t generals/cni-terway:1.0 .
########################################################
FROM golang:1.12 as builder
## docker镜像通用设置
LABEL author=general
LABEL email="generals.space@gmail.com"
## 环境变量, 使docker容器支持中文
ENV LANG C.UTF-8

WORKDIR /cni-terway
COPY . .
ENV GO111MODULE on
RUN go build -o cni-terway ./
########################################################
FROM generals/alpine
## docker镜像通用设置
LABEL author=general
LABEL email="generals.space@gmail.com"
## 环境变量, 使docker容器支持中文
ENV LANG C.UTF-8

COPY --from=builder /cni-terway/cni-terway /
CMD ["/cni-terway"]
