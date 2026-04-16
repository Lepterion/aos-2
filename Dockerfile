FROM golang:1.25.3 AS build

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 go build -o /out/aos_2 ./main.go

FROM ubuntu:24.04 AS runtime

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
       ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/aos_2 /usr/local/bin/aos_2
COPY code.txt /app/code.txt
WORKDIR /app

CMD ["aos_2"]
