# shine-mp3

This is a pure Go implementation of the [shine mp3 encoding library](https://github.com/toots/shine).

> shine is a blazing fast mp3 encoding library implemented in fixed-point arithmetic. The library can thus be used to perform super fast mp3 encoding on architectures without a FPU, such as armel, etc.. It is also super fast on architectures with a FPU!

AFAIK, this is the only pure Go MP3 **_encoding_** library. This project currently produces byte-identical binaries to the original Shine C library.

The industry leading LAME MP3 encoding library [says this about Shine](https://lame.sourceforge.io/links.php):

> [Shine](https://www.mp3-tech.org/programmer/encoding.html) is a featureless, but clean and readable MP3 encoder by Gabriel Bouvigne of LAME fame. Great as a starting point or learning tool. Also probably the only open source, fixed point math MP3 encoder.

This means `shine-mp3` will produce compressed files that are valid MP3 files and sound good, but they will:

- be larger than MP3 files produced by better libraries (i.e. LAME)
- be of less quality than MP3 files produced by better libraries (i.e. LAME)
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

## Development

The Dockerfile is provided. It builds and install both the original C library and `shine-mp3`:

```bash
# Create the container
docker build -t shine .
# Convert a WAV file with Go implementation
docker run -it --rm -v $(pwd):$(pwd) -w $(pwd) shine-encoder shine-mp3 testdata/test.wav test.Go.mp3
# Convert it with the original C implementation
docker run -it --rm -v $(pwd):$(pwd) -w $(pwd) shine-encoder shineenc testdata/test.wav test.C.mp3
# Confirm the same files were created
md5sum *.mp3
c255da6921a5a726547ce6a323dfe95e  test.C.mp3
c255da6921a5a726547ce6a323dfe95e  test.Go.mp3
```

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
