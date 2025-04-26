package main

import (
	"flag"
	"fmt"
	"github.com/pmdroid/microwakeword/pkg/audio"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MarkKremer/microphone/v2"
	"github.com/pmdroid/microwakeword"
)

const (
	SampleRate = 16000
)

func main() {
	modelsDir := flag.String("models", "./models", "Directory containing the wake word models")
	wakeWordName := flag.String("wakeword", "okay_nabu", "Name of the wake word model to use")
	flag.Parse()

	if _, err := os.Stat(*modelsDir); os.IsNotExist(err) {
		log.Fatalf("Models directory does not exist: %s", *modelsDir)
	}

	wakeWord, err := microwakeword.FromBuiltin(
		*wakeWordName,
		*modelsDir,
		microwakeword.DefaultRefractory,
	)
	if err != nil {
		log.Fatalf("Failed to load builtin model: %v", err)
	}

	err = microphone.Init()
	if err != nil {
		log.Fatalf("Failed to initialize microphone: %v", err)
	}
	defer microphone.Terminate()

	stream, _, err := microphone.OpenDefaultStream(SampleRate, 1)
	if err != nil {
		log.Fatalf("Failed to create microphone stream: %v", err)
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		log.Fatalf("Failed to start microphone stream: %v", err)
		return
	}

	fmt.Printf("Listening for wake word: '%s'\n", *wakeWordName)
	fmt.Println("Press Ctrl+C to exit")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		audioBuffer := make([][2]float64, 160)

		for {
			numSamples, _ := stream.Stream(audioBuffer)
			audioBytes := audio.ToLittleEndian(audioBuffer, numSamples)

			result, err := wakeWord.ProcessStreaming(audioBytes)
			if err != nil {
				log.Printf("Error processing audio: %v\n", err)
				continue
			}

			if result {
				timestamp := time.Now().Format("15:04:05")
				fmt.Printf("[%s] Wake word '%s' detected!\n", timestamp, *wakeWordName)
			}
		}
	}()

	<-signalChan
	fmt.Println("\nStopping...")
}
