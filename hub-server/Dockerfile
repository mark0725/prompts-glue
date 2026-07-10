FROM golang:1.26 AS build
WORKDIR /src
COPY prompts-sdk /src/prompts-sdk
COPY hub-server /src/hub-server
WORKDIR /src/hub-server
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/hub-server .

FROM alpine:latest

COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo                                                                                                                                                                                                                                             
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt                                                                                                                                                                                                               

RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo "Asia/Shanghai" > /etc/timezone
WORKDIR /app

COPY --from=build /out/hub-server /app/hub-server
COPY hub-server/config.example.yaml /app/config.yaml
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 \
  CMD ["/app/hub-server", "healthcheck", "--config", "/app/config.yaml"]
ENTRYPOINT ["/app/hub-server"]
CMD ["--config", "/app/config.yaml"]
