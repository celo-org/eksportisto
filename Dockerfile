FROM golang:1.13-alpine as builder
WORKDIR /app
RUN apk add --no-cache make git gcc linux-headers musl-dev

# Download dependencies & cache them in docker layer
COPY go.mod .
COPY go.sum .
RUN go mod download

# Build project (this prevents re-downloading dependencies when go.mod/sum didn't change)
COPY . .
RUN go build -tags musl -o eksportisto .

FROM scratch

ENV HOME /home
COPY --from=builder /app/eksportisto /app/eksportisto

ENTRYPOINT [ "/app/eksportisto" ]