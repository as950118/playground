FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /go-agent .

FROM alpine:3.20
COPY --from=builder /go-agent /go-agent
ENTRYPOINT ["/go-agent"]
