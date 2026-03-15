FROM golang:alpine AS bulder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /tournament
WORKDIR /
CMD ["/tournament"]