# BUILD STAGE

FROM golang:1.26.1-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o TickerAnalyzer .

# Production

FROM alpine:latest
WORKDIR /root/

COPY --from=builder /app/TickerAnalyzer .
COPY --from=builder /app/index.html .

EXPOSE 8080

# run server
CMD ["./TickerAnalyzer"]