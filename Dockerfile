FROM golang:1.15.5-alpine3.12

WORKDIR /app

COPY . ./

RUN go get -d -v ./...
RUN go build -o ./main ./cmd/main.go

RUN chmod u+x ./main

ENV FRONTEND_URL=https://downloadsound.cloud

CMD ["./main"]