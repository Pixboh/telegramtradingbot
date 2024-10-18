FROM golang:1.19-alpine as builder

RUN mkdir "/build"
ADD . "/build/"
WORKDIR "/build"
RUN go build -o tdlib

FROM alpine
RUN mkdir "/app"
WORKDIR "/app"
COPY --from=builder /build/ /app/

CMD ["./tdlib"]