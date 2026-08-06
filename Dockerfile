FROM golang:1.25.12-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/l402test ./cmd/l402test

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/l402test /l402test
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/l402test"]
