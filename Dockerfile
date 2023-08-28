FROM golang:1.20
ADD . /go/src/github.com/m-lab/locate
WORKDIR /go/src/github.com/m-lab/locate
RUN go get -v github.com/m-lab/locate && \
    go install -v \
        -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h)" \
        github.com/m-lab/locate
ENTRYPOINT ["/go/bin/locate"]
