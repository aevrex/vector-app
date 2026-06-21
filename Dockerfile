FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /vector .

FROM alpine:3.22

WORKDIR /app

RUN mkdir -p /data

COPY --from=build /vector /app/vector
COPY templates /app/templates

ENV DB_PATH=/data/vector.db

EXPOSE 8080

CMD ["/app/vector"]
