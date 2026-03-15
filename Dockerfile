FROM golang:alpine
ENV TG_BOT_TOKEN=token
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /tournament
WORKDIR /
CMD ["/tournament"]