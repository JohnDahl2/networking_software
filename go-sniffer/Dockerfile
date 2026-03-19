FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y libpcap-dev

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o sniffer main.go

CMD ["./sniffer"]