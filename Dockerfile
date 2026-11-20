# The toolchain is what patches the standard library in the shipped binary, so
# this line is the fix for the net/http, crypto/tls, crypto/x509, net/textproto,
# encoding/xml and encoding/asn1 advisories govulncheck reported. go.mod's
# directive does not affect that. 1.23 had fallen out of Go's two-release support
# window, so its stdlib was unpatched rather than merely behind.
FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bondcalc ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bondcalc /bondcalc
EXPOSE 8080
ENTRYPOINT ["/bondcalc"]
