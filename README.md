# shine-mp3

This is a pure Go implementation of the [shine mp3 encoding library](https://github.com/toots/shine).

> shine is a blazing fast mp3 encoding library implemented in fixed-point arithmetic. The library can thus be used to perform super fast mp3 encoding on architectures without a FPU, such as armel, etc.. It is also super fast on architectures with a FPU!

AFAIK, this is the only pure Go MP3 **_encoding_** library. This project currently produces byte-identical binaries to the original Shine C library.

Industry-leading LAME MP3 encoding library [says this about Shine](https://lame.sourceforge.io/links.php):

> [Shine](https://www.mp3-tech.org/programmer/encoding.html) is a featureless, but clean and readable MP3 encoder by Gabriel Bouvigne of LAME fame. Great as a starting point or learning tool. Also probably the only open source, fixed point math MP3 encoder.

This means `shine-mp3` will produce compressed files that are valid MP3 files and sound good, but they will:

- be of lower quality than MP3 files produced by better libraries (i.e. LAME) at the same bitrate, due to lack of perceptual modeling
- not benefit from perceptual encoding that could allow for smaller files at equivalent quality
- contain no ID3 metadata

## Usage

The `main.go` file has simple example of reading WAV file and encoding it to MP3. It all comes down to this:

```go
// Create the encoder with the sample rate and number of audio channels
mp3Encoder := NewEncoder(44100, 2)
// Assuming all your audio data is in []int16 slice called decodedData, write it to a file referenced by out
mp3Encoder.Write(out, decodedData)
```

## A Quick Comment on MP3 Encoders

There is essentially one actively maintained MP3 encoding library: [LAME MP3](https://lame.sourceforge.io/). If you want to encode audio files to MP3 using a programming language, you use a library that provides bindings to LAME, which means users of your software must have the LAME MP3 C library installed. You might be able to work around this by producing a 100% statically compiled binary and including the LAME files inside. However, that can prove challenging on all platforms, especially Windows.

I found about the Shine MP3 encoder from this [list of alternative encoders on the LAME website](https://lame.sourceforge.io/links.php#Alternatives).

## MP3 Encoder Algorithm

I spent quite a lot of time researching MP3 and here's what I found:

- There is no encoder specification
    - The [Library of Congress](https://www.loc.gov/preservation/digital/formats/fdd/fdd000012.shtml) is a good place to start. It references ISO docs [13818-3](https://www.iso.org/standard/26797.html) and [11172-3](https://www.iso.org/standard/22412.html) but they are behind paywalls, so essentially a dead-end for hobbyist like me.
    - From [Wikipedia](https://www.wikiwand.com/en/MP3#Encoding_and_decoding),
    > The MPEG-1 standard does not include a precise specification for an MP3 encoder but does provide examples of psychoacoustic models, rate loops, and the like in the non-normative part of the original standard...When this was written, the suggested implementations were quite dated. Implementers of the standard were supposed to devise algorithms suitable for removing parts of the information from the audio input. As a result, many different MP3 encoders became available, each producing files of differing quality
- MP3 was licensed [until 2017](https://patents.google.com/patent/US6009399), stifling access, interest, and innovation

I archived documents that helped:

- [MP3 Theory](./docs/mp3_theory.pdf): Start here. It introduces all the terms and the algorithm at a high, but still detailed, level
- [Analysis of the MPEG-1 Layer III MP3 Algorithm using Matlab](./docs/analysis-of-the-mpeg-1layer-iii-mp3-algorithm-using-matlab.pdf): A deeper look into the algorithm, with corresponding Matlab code implementations

## WebAssembly Example

The project includes a WebAssembly example that demonstrates using the encoder in a web browser. You can find it in the `wasm_example` directory.

### Running the Example

1. Build and run the example:

```bash
make run
```

2. Open http://localhost:8080 in your browser
3. Select a WAV file using the file input
4. The file will be converted to MP3 entirely in your browser
5. Click the download link to save the converted MP3

### How it Works

The example demonstrates:

- Compiling the Go MP3 encoder to WebAssembly
- Using the WebAssembly module in a browser
- Converting WAV files to MP3 without server-side processing

The key files are:

- `wasm_example/main.go`: The Go code that exposes the encoder to JavaScript
- `wasm_example/index.html`: A minimal web interface for file conversion
- `Makefile`: Build targets for compiling and serving the example

This is a practical demonstration of running computationally intensive audio processing directly in the browser using WebAssembly.

## Performance

A Dockerfile is provided to quickly asses the quality of produced MP3s for comparison. It builds and installs the original C library, `shine-mp3`, and LAME.

```bash
# Create the container
docker build -t shine .
# Convert a WAV file with Go implementation
docker run -it --rm -v $(pwd):$(pwd) -w $(pwd) shine shine-mp3 testdata/test.wav test.Go.mp3
# Convert it with the original C implementation
docker run -it --rm -v $(pwd):$(pwd) -w $(pwd) shine shineenc testdata/test.wav test.C.mp3
# Confirm the same files were created
md5sum *.mp3
c255da6921a5a726547ce6a323dfe95e  test.C.mp3
c255da6921a5a726547ce6a323dfe95e  test.Go.mp3
```

### Comparing All Three Encoders

Use the `compare-encoders` command to encode a WAV file with all three implementations:

```bash
# Compare Shine C, Shine Go, and LAME
docker run -it --rm -v $(pwd):/data shine compare-encoders testdata/test.wav

# This creates three files:
#   testdata/test-shine-c.mp3   (Shine C)
#   testdata/test-shine-go.mp3  (Shine Go)
#   testdata/test-lame.mp3      (LAME)
```

The output shows file sizes for comparison.

### Benchmarks

Here's a rough measurement of the quality and performance of encoder. In this methodology, the Go encoder is compared to the C implementation of Shine. Lame is also in there as a golden standard. To measure perceptual quality, [`visqol`](https://github.com/google/visqol) is used.

### Baseline

The initial port without a psychoacoustics model.

```text
Encoder              Size         Time         Bitrate        MD5
-------              ----         ----         -------        ---
Shine C              63K          .021363543  s 128.32         c255da6921a5a726547ce6a323dfe95e
Shine Go             63K          .051710349  s 128.32         c255da6921a5a726547ce6a323dfe95e
LAME                 64K          .051859307  s 129.85         5829fa1a6af23c2efdf3ea22c074af80
```

MOS-LQO:                4.60448

### Minimal Perceptual Coding

```text
Encoder              Size         Time         Bitrate        MD5
-------              ----         ----         -------        ---
Shine C              63K          .021788051  s 128.32         c255da6921a5a726547ce6a323dfe95e
Shine Go             63K          .055918262  s 128.32         aea176a76c35d2bd7b83005547ad4a74
LAME                 64K          .049341680  s 129.85         5829fa1a6af23c2efdf3ea22c074af80
```

```text
MOS-LQO:                4.60448
```
