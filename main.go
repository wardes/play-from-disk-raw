package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pion/webrtc/v3"

	"github.com/pion/webrtc/v3/pkg/media"

	"gopkg.in/hraban/opus.v2"
)

const (
	audioFileName = "output.raw"
	sampleRate    = 48000
	channels      = 2
)

func main() {
	// Assert that we have an audio or video file

	_, err := os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile {
		panic("Could not find `" + audioFileName)
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	if haveAudioFile {
		// Create a audio track
		audioTrack, audioTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion")
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		rtpSender, audioTrackErr := peerConnection.AddTrack(audioTrack)
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		// Read incoming RTCP packets
		// Before these packets are retuned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		go func() {
			f, err := os.Open(audioFileName)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			//Create Opus Encoder
			enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
			if err != nil {
				panic(err)
			}

			// Wait for connection established
			<-iceConnectedCtx.Done()

			ticker := time.NewTicker(20 * time.Millisecond)
			//done := make(chan bool)

			for {
				// Opus encoding looping
				// Opus frame size must be: 2.5, 5, 10, 20, 40 or 60 ms long
				// 20ms @48000hz gives 960 samples per channel => 1920 floats
				var pcm []float32 = make([]float32, 1920)
				var raw []byte = make([]byte, 1920*4)

				var bytes_read int
				//read this from the raw file
				bytes_read, err = f.Read(raw)
				if err != nil {
					log.Printf("read raw bytes error: %v", err)
				}
				buffer := bytes.NewBuffer(raw)
				binary.Read(buffer, binary.LittleEndian, &pcm)

				if bytes_read != 1920*4 {
					log.Printf("Only read %v bytes from file... expected %d", bytes_read, 1920*4)
				}

				data := make([]byte, 1000) //where to put compressed opus bytes

				n, err := enc.EncodeFloat32(pcm, data)
				if err != nil {
					log.Printf("enc.EncodeFloat32 err: %v", err)
					panic(err)
				} else {
					log.Printf("enc.EncodeFloat32 encoded to %v bytes", n /*, data*/)
				}

				data = data[:n] //truncate to actual amount used to encode
				sampleDuration := time.Duration(20 * time.Millisecond)

				sample := media.Sample{Data: data, Duration: sampleDuration}

				//wait for ticker...
				<-ticker.C

				err = audioTrack.WriteSample(sample)
				if err != nil {
					log.Printf("audioTrack.WriteSample error: %v", err)
					panic(err)
				}
				//time.Sleep(sampleDuration)
			}
		}()
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	Decode(MustReadStdin(), &offer)

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(Encode(*peerConnection.LocalDescription()))

	// Block forever
	select {}
}
