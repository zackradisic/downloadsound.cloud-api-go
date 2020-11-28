FROM golang:1.15.5-alpine3.12

WORKDIR /go/src/app

COPY . ./

RUN go get -d -v ./...
RUN go build -o /go/src/app/server ./cmd/main.go

ENV FRONTEND_URL=https://downloadsound.cloud

CMD ["/go/src/app/server"]