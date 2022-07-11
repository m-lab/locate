FROM golang:1.18
ADD . /go/src/github.com/m-lab/locate
WORKDIR /go/src/github.com/m-lab/locate
RUN go get -v github.com/m-lab/locate && \
    go install -v github.com/m-lab/locate
ENTRYPOINT ["/go/bin/locate"]
