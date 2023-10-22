## BUILD ######
FROM golang:1.21.3-alpine as build

WORKDIR /temp
COPY . ./
RUN go mod download

WORKDIR /temp/application/web
RUN go build -o ./build/counter ./main.go


## RUN ######
FROM alpine:latest

WORKDIR /app
COPY --from=build /temp/application/web/build/counter ./
COPY --from=build /temp/application/web/template ./template

RUN chmod +x ./counter

EXPOSE 3333
ENTRYPOINT /app/counter

