FROM golang:alpine
AS build
WORKDIR /go/src/github.com/LGUG2Z/triforce
COPY . /go/src/github.com/LGUG2Z/triforce
RUN go build -o /usr/local/bin/triforce

FROM alpine
COPY --from=build /usr/local/bin/triforce /usr/local/bin
ENTRYPOINT ["triforce"]
