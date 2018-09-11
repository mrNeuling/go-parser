FROM golang:1.10.4-alpine

RUN apk add git
RUN go get github.com/PuerkitoBio/goquery
RUN go get github.com/mongodb/mongo-go-driver/mongo

WORKDIR /go/src