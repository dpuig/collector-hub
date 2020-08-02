# BUILD
FROM golang:1.14.6-alpine3.12 as builder

# All these steps will be cached
RUN mkdir /app
WORKDIR /app
COPY go.mod .
COPY go.sum .

# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download
# COPY the source code as the last step
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/collector

FROM scratch 
COPY --from=builder /go/bin/collector /go/bin/collector
ENTRYPOINT ["/go/bin/collector"]