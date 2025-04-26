# MicroWakeWord

A Golang library for wake word detection using TensorFlow Lite and audio microfrontend processing.

## Overview

MicroWakeWord is a lightweight wake word detection library for Go applications. It leverages TensorFlow Lite and the audio microfrontend to provide efficient and accurate wake word detection capabilities with minimal resource usage, making it suitable for embedded systems and IoT devices.

This library is inspired by [microWakeWord](https://github.com/kahrendt/microWakeWord) and provides a simple API for detecting predefined wake words from audio input.

## Prerequisites

- Go 1.16 or higher
- GCC or compatible C compiler
- Bazel build system
- Git

## Installation

### 1. Build Dependencies

The library requires two main dependencies:
- TensorFlow Lite C library
- Audio Microfrontend library

Use the provided Makefile to build and install these dependencies:

```bash
# Clone the repository
git clone https://github.com/pmdroid/microwakeword.git
cd microwakeword

# Build and install dependencies (may require sudo for installation)
make
```

The Makefile will:
1. Download TensorFlow v2.19.0
2. Download KissFFT
3. Build the TensorFlow Lite C and Microfrontend libraries
4. Install the shared libraries to `/usr/local/lib`

### 2. Install the Go Library

```bash
go get github.com/pmdroid/microwakeword
```

## Usage

### Examples

The repository includes examples to help you get started:

- `examples/mic.go`: A complete example showing how to use the library with microphone input for real-time wake word detection

The microphone example demonstrates:
- Loading a wake word model
- Initializing and configuring the microphone
- Processing streaming audio data
- Detecting wake words in real-time

To run the microphone example:

```bash
go run examples/mic.go
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgements

- This project uses [TensorFlow](https://github.com/tensorflow/tensorflow)
- KissFFT library by Mark Borgerding