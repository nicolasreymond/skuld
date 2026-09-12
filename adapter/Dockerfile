FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /adapter .

FROM alpine:3
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /adapter /adapter
EXPOSE 8080
ENTRYPOINT ["/adapter"]
