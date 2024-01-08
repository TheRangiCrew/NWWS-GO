FROM golang:1.21.5

# Set destination for COPY
WORKDIR /

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY *.go ./
COPY internal/*.go internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o nwws-go

# Run
CMD [ "./nwws-go" ]