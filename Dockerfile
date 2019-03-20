FROM golang as builder

WORKDIR /go/src/github.com/toyo/epsp
COPY . .
RUN cd cmd/p2pquake && go get -d -v ./... && CGO_ENABLED=0 go build -o app 

FROM alpine
#RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/src/github.com/toyo/epsp/cmd/p2pquake/app ./app
COPY --from=builder /go/src/github.com/toyo/epsp/cmd/p2pquake/html/index.html ./html/index.html
COPY --from=builder /go/src/github.com/toyo/epsp/cmd/p2pquake/html/635.html ./html/635.html
ENTRYPOINT ["./app","-d"]
VOLUME ["/tmp"]

EXPOSE 6980:6980
EXPOSE 6911:6911

ENV PORT 6980
