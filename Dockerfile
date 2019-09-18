FROM golang:1.12

WORKDIR /src/gke-node-drainer/

ENV GO111MODULE on

COPY . .

RUN go mod tidy


RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gke-node-drainer .

FROM alpine

ENV GOOGLE_APPLICATION_CREDENTIALS /etc/credentials/credentials.json

RUN apk add -U --no-cache ca-certificates

COPY --from=0 /src/gke-node-drainer /

CMD ["/gke-node-drainer"]
