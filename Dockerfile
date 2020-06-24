FROM golang:1.14-alpine AS builder
COPY . /app/
WORKDIR /app
RUN go build -o app

FROM maxisme/megatools-alpine
RUN apk add --update rsync openssh curl bash unzip tar
RUN curl https://rclone.org/install.sh | bash -s beta
RUN apk del zip bash

WORKDIR /app
COPY . /app/
COPY --from=builder /app/app /app/app
CMD ["/app/app"]