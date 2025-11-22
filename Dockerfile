FROM golang:bookworm

WORKDIR /app

# Update and install necessary dependencies
RUN apt-get update && \
    apt-get install -y build-essential curl lame bc ffmpeg && \
    rm -rf /var/lib/apt/lists/*

# Download Shine C source code
RUN curl -LO https://github.com/toots/shine/releases/download/3.1.1/shine-3.1.1.tar.gz && \
    tar -xzf shine-3.1.1.tar.gz && \
    rm shine-3.1.1.tar.gz

# Build Shine C
WORKDIR /app/shine-3.1.1
RUN ./configure && \
    make install
ENV LD_LIBRARY_PATH=/usr/local/lib

# Copy in shine-mp3 Go source code
WORKDIR /app/shine-mp3
COPY . .

# Build/install shine-mp3 Go
RUN go install

# Create comparison script
RUN cat <<'EOF' > /usr/local/bin/compare-encoders
#!/bin/bash
set -e

if [ "$#" -ne 1 ]; then
    echo "Usage: compare-encoders <input.wav>"
    echo ""
    echo "This will create three MP3 files:"
    echo "  - <input>-shine-c.mp3    (Shine C implementation)"
    echo "  - <input>-shine-go.mp3   (Shine Go implementation)"
    echo "  - <input>-lame.mp3       (LAME encoder)"
    exit 1
fi

INPUT="$1"
BASENAME="${INPUT%.wav}"

if [ ! -f "$INPUT" ]; then
    echo "Error: File not found: $INPUT"
    exit 1
fi

calc_bitrate() {
    local file="$1"
    local duration="$2"
    local bytes=$(stat -c%s "$file")
    if [ -z "$duration" ] || [ "$duration" = "0" ]; then
        echo "n/a"
        return
    fi
    local kbps=$(echo "scale=4; ($bytes * 8) / $duration / 1000" | bc -l)
    printf "%.2f" "$kbps"
}

DURATION=$(ffprobe -v error -show_entries format=duration -of default=nk=1:nw=1 "$INPUT")

echo "=== Encoding with Shine C ==="
START=$(date +%s.%N)
shineenc "$INPUT" "${BASENAME}-shine-c.mp3"
END=$(date +%s.%N)
SHINE_C_TIME=$(echo "$END - $START" | bc)
echo "Time: ${SHINE_C_TIME}s"

echo ""
echo "=== Encoding with Shine Go ==="
START=$(date +%s.%N)
shine-mp3 "$INPUT" "${BASENAME}-shine-go.mp3"
END=$(date +%s.%N)
SHINE_GO_TIME=$(echo "$END - $START" | bc)
echo "Time: ${SHINE_GO_TIME}s"

echo ""
echo "=== Encoding with LAME ==="
START=$(date +%s.%N)
lame "$INPUT" "${BASENAME}-lame.mp3"
END=$(date +%s.%N)
LAME_TIME=$(echo "$END - $START" | bc)
echo "Time: ${LAME_TIME}s"

SHINE_C_BITRATE=$(calc_bitrate "${BASENAME}-shine-c.mp3" "$DURATION")
SHINE_GO_BITRATE=$(calc_bitrate "${BASENAME}-shine-go.mp3" "$DURATION")
LAME_BITRATE=$(calc_bitrate "${BASENAME}-lame.mp3" "$DURATION")

echo ""
echo "=== Results ==="
printf "%-20s %-12s %-12s %-14s %-34s\n" "Encoder" "Size" "Time" "Bitrate" "MD5"
printf "%-20s %-12s %-12s %-14s %-34s\n" "-------" "----" "----" "-------" "---"
printf "%-20s %-12s %-12ss %-14s %-34s\n" "Shine C" "$(ls -lh ${BASENAME}-shine-c.mp3 | awk '{print $5}')" "$SHINE_C_TIME" "${SHINE_C_BITRATE}" "$(md5sum ${BASENAME}-shine-c.mp3 | awk '{print $1}')"
printf "%-20s %-12s %-12ss %-14s %-34s\n" "Shine Go" "$(ls -lh ${BASENAME}-shine-go.mp3 | awk '{print $5}')" "$SHINE_GO_TIME" "${SHINE_GO_BITRATE}" "$(md5sum ${BASENAME}-shine-go.mp3 | awk '{print $1}')"
printf "%-20s %-12s %-12ss %-14s %-34s\n" "LAME" "$(ls -lh ${BASENAME}-lame.mp3 | awk '{print $5}')" "$LAME_TIME" "${LAME_BITRATE}" "$(md5sum ${BASENAME}-lame.mp3 | awk '{print $1}')"
EOF
RUN chmod +x /usr/local/bin/compare-encoders

WORKDIR /data
