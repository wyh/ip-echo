FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /ip-echo .

FROM scratch
COPY --from=builder /ip-echo /ip-echo
EXPOSE 18080
ENTRYPOINT ["/ip-echo"]
