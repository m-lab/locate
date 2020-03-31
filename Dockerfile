FROM golang:1.13
ADD . /go/src/github.com/m-lab/locate
WORKDIR /go/src/github.com/m-lab/locate
RUN go get -v github.com/m-lab/locate
ENTRYPOINT ["/go/bin/locate"]
