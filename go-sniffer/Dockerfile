FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y libpcap-dev

RUN go install github.com/pressly/goose/v3/cmd/goose@v3.21.1
WORKDIR /app
COPY . .
RUN go mod download
RUN go mod tidy
RUN go build -o sniffer .

CMD ["./sniffer"]