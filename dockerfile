FROM golang AS BUILD

WORKDIR /proxy

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build

CMD ["./start.sh"]
