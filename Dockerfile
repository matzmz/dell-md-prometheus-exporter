FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o /out/dell-md-exporter ./cmd/dell_md_exporter

FROM scratch

COPY --from=builder /out/dell-md-exporter /dell-md-exporter

ENTRYPOINT ["/dell-md-exporter"]
