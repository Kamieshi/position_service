FROM golang:1.18 as builde

RUN mkdir build

WORKDIR /build

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build main.go

FROM alpine:latest

COPY --from=builde build/main .

EXPOSE 5301/tcp
CMD ["./main"]