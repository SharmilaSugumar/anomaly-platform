package pipeline

import (
	"bufio"
	"context"
	"io"
	"log"
	"os/exec"
	"time"

	"github.com/sharm/anomaly-platform/internal/models"
)

func MockFrameSource(
	ctx context.Context,
	videoPath string,
	camID string,
	out chan<- models.Frame,
	skipN int,
) {
	frameCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("[Mock] Starting video loop: %s", videoPath)

		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-loglevel", "quiet",
			"-i", videoPath,
			"-vf", "scale=640:640",
			"-f", "image2pipe",
			"-vcodec", "mjpeg",
			"-r", "10",
			"pipe:1",
		)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("[Mock] FFmpeg pipe error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := cmd.Start(); err != nil {
			log.Printf("[Mock] FFmpeg start error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		scanner := newJPEGScanner(stdout)
		for {
			select {
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			default:
			}

			jpegBytes, err := scanner.Next()
			if err != nil || jpegBytes == nil {
				break
			}

			frameCount++
			if skipN > 0 && frameCount%skipN != 0 {
				continue
			}

			frame := models.Frame{
				Data:  jpegBytes,
				CamID: camID,
				TS:    time.Now(),
			}

			select {
			case out <- frame:
			default:
			}
		}

		cmd.Wait()
		log.Printf("[Mock] Video ended, restarting loop...")
		time.Sleep(100 * time.Millisecond)
	}
}

type jpegScanner struct {
	reader *bufio.Reader
	buf    []byte
}

func newJPEGScanner(r io.Reader) *jpegScanner {
	return &jpegScanner{reader: bufio.NewReaderSize(r, 1<<20)}
}

func (s *jpegScanner) Next() ([]byte, error) {
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != 0xFF {
			continue
		}
		next, err := s.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if next == 0xD8 {
			s.buf = []byte{0xFF, 0xD8}
			break
		}
	}
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		s.buf = append(s.buf, b)
		if len(s.buf) >= 2 {
			tail := s.buf[len(s.buf)-2:]
			if tail[0] == 0xFF && tail[1] == 0xD9 {
				result := make([]byte, len(s.buf))
				copy(result, s.buf)
				s.buf = s.buf[:0]
				return result, nil
			}
		}
	}
}