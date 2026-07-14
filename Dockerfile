FROM golang:1.26.5-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.23

RUN adduser -D -H app && mkdir /data && chown app:app /data
USER app
ENV DATABASE_URL=/data/grpc-demo.db
COPY --from=build /out/server /usr/local/bin/server
EXPOSE 8080 8081
ENTRYPOINT ["server"]
