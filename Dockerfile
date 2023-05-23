FROM alpine as build

# install base build tools
RUN apk add build-base go 

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download -x

COPY . .

RUN GOARCH=amd64 GOOS=linux go build -o test ./cmd/test

#
# final exported image
#
FROM alpine
WORKDIR /app
COPY --from=build /app/test test

VOLUME [ "/root/.aws" ]

ENTRYPOINT [ "./test" ]