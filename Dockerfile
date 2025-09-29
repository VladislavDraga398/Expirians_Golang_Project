FROM golang:1.22.4

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . .

RUN go build -o main main.go

CMD ["./main"]
    