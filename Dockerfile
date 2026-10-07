FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN wget -q https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem -O /out-rds-ca.pem
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/provision ./cmd/provision

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/ /app/
COPY --from=build /out-rds-ca.pem /app/rds-ca.pem
ENV PGSSLROOTCERT=/app/rds-ca.pem
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["/app/api"]
