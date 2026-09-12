# Image officielle gaps-cli = amd64 seulement → on compile depuis les sources pour l'archi de l'hôte (arm64).
FROM golang:1.23-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
RUN git clone --depth 1 https://github.com/heig-lherman/gaps-cli . \
    && CGO_ENABLED=0 go build -ldflags="-s -w" -o /gaps-cli .

FROM alpine:3
RUN apk add --no-cache ca-certificates
COPY --from=build /gaps-cli /gaps-cli
ENV GAPS_HISTORY_GRADES_FILE=/history/grades.json
ENTRYPOINT ["/gaps-cli"]
CMD ["--help"]
