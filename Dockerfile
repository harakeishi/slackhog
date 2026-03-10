FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /slackhog .

FROM scratch
COPY --from=builder /slackhog /slackhog
EXPOSE 4112
ENTRYPOINT ["/slackhog"]
