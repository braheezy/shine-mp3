FROM golang:bookworm

WORKDIR /app

# Update and install necessary dependencies
RUN apt-get update && \
    apt-get install -y build-essential curl && \
    rm -rf /var/lib/apt/lists/*

# Download Shine C source code
RUN curl -LO https://github.com/toots/shine/releases/download/3.1.1/shine-3.1.1.tar.gz && \
    tar -xzf shine-3.1.1.tar.gz && \
    rm shine-3.1.1.tar.gz

# Build Shine
WORKDIR /app/shine-3.1.1
RUN ./configure && \
    make install
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib

# Copy in shine-mp3 Go source code
WORKDIR /app/shine-mp3
COPY . .

# Build/install shine-mp3
RUN go install
