FROM golang AS build

WORKDIR /proxy

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY start.sh .
COPY internal internal
COPY api api
COPY xdp xdp
COPY proxy.go .

RUN GOAMD64=v3 go build

FROM ubuntu

COPY --from=build /proxy/gone-proxy /gone-proxy
COPY --from=build /proxy/start.sh /start.sh

CMD ["/start.sh"]
