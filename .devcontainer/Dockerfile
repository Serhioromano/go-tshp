FROM mcr.microsoft.com/devcontainers/go:0-1-bullseye

WORKDIR /workspaces/gotest

COPY . .

RUN apt update && \
    apt upgrade -y && \
    git config --global user.name "Serhioromano" && \
    git config --global user.email "serg4172@mail.ru"

RUN cd /workspaces/gotest/app
RUN go get github.com/githubnemo/CompileDaemon && \
    go install github.com/githubnemo/CompileDaemon
RUN go get -v ./...
RUN go install -v ./...

CMD ["app"]
