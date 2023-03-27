FROM golang:1.19

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /ipfs-metric-proxy

EXPOSE 9100

CMD [ "/ipfs-metric-proxy" ]