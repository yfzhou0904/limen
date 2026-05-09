# --- ui build ---
FROM node:20-alpine AS ui
WORKDIR /app/ui
COPY ui/package.json ui/package-lock.json* ./
RUN npm ci || npm install
COPY ui/ ./
RUN npm run build

# --- go build ---
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY server/ ./server/
COPY --from=ui /app/server/dist ./server/dist
RUN CGO_ENABLED=0 go build -o /out/limen ./server

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=server /out/limen /usr/local/bin/limen
ENV ADDR=:8080
ENV DATA_DIR=/data
EXPOSE 8080
CMD ["/usr/local/bin/limen"]
