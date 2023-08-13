FROM golang:1.21 AS BUILD

WORKDIR /go/src/app
COPY . .
RUN make

FROM gcc:13
COPY --from=BUILD /go/src/app/bin/executor /usr/bin/executor
COPY ./configs/executor.yaml /etc/all-build/executor.yaml
CMD ["/usr/bin/executor"]
