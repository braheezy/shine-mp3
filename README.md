# shine-mp3
This is a pure Go implementation of the [shine mp3 encoding library](https://github.com/toots/shine). AFAIK, this is the only pure Go MP3 ***encoding*** library.

> shine is a blazing fast mp3 encoding library implemented in fixed-point arithmetic. The library can thus be used to perform super fast mp3 encoding on architectures without a FPU, such as armel, etc.. It is also super fast on architectures with a FPU!

The industry leading LAME MP3 encoding library [says this about Shine](https://lame.sourceforge.io/links.php):
> [Shine](https://www.mp3-tech.org/programmer/encoding.html) is a featureless, but clean and readable MP3 encoder by Gabriel Bouvigne of LAME fame. Great as a starting point or learning tool. Also probably the only open source, fixed point math MP3 encoder.

This project currently produces byte-identical binaries to the original Shine C library.

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
A Dockerfile has been created for convenience. It builds and install both the original C library and `shine-mp3`:

    # Create the container
    docker build -t shine .
    # Convert a WAV file with `shine-mp3`
    docker run -it --rm -v $(pwd):$(pwd) -w $(pwd) shine-encoder shine-mp3 testdata/test.wav test.Go.mp3
    # Convert it with the original C implementation
    docker run -it --rm -v $(pwd):$(pwd) -w $(pwd) shine-encoder shineenc testdata/test.wav test.C.mp3
    # Confirm you created the same thing
    md5sum *.mp3
    c255da6921a5a726547ce6a323dfe95e  test.C.mp3
    c255da6921a5a726547ce6a323dfe95e  test.Go.mp3
