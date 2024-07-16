package gphotosfb

import (
	"context"
  "errors"
	"fmt"
	"image"
	"image/draw"
	"math/rand"
	"net/http"
	"os"
	"log"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	gphotos "github.com/gphotosuploader/google-photos-api-client-go/v3"
	mediaitems "github.com/gphotosuploader/google-photos-api-client-go/v3/media_items"
	"github.com/zenhack/framebuffer-go"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
)

type idList struct {
	mut  sync.RWMutex
	ids []string
	rng  *rand.Rand
}

func newIDList() *idList {
	return &idList{
		ids: make([]string, 0, 1024),
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (l *idList) rand() (string, bool) {
	l.mut.RLock()
	defer l.mut.RUnlock()

	if len(l.ids) == 0 {
		return "", false
	}

	return l.ids[l.rng.Intn(len(l.ids)-1)], true
}

func (l *idList) push(ids ...string) {
	l.mut.Lock()
	defer l.mut.Unlock()
	l.ids = append(l.ids, ids...)
}

type slideshow struct {
  client *gphotos.Client
  library *idList
  width uint16
  height uint16
}

func newSlideShow(client *gphotos.Client, width, height uint16) *slideshow {
  return &slideshow{
    client: client,
    library: newIDList(),
    width: width,
    height: height,
  }
}

func (s *slideshow) loadLibrary(ctx context.Context) error {
	var pageToken string
	for {
		items, nextToken, err := s.client.MediaItems.PaginatedList(ctx, &mediaitems.PaginatedListOptions{
			PageToken: pageToken,
		})
		if err != nil {
			return fmt.Errorf("photoslibraryService.MediaItems.List: %w", err)
		}
		if nextToken == "" {
			break
		}
		pageToken = nextToken

		ids := make([]string, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}
		s.library.push(ids...)
	}

	return nil
}

func (s *slideshow) nextImage(ctx context.Context) (image.Image, error) {
	id, ok := s.library.rand()
	if !ok {
		return nil, errors.New("empty library")
	}

	mediaItem, err := s.client.MediaItems.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("s.client.MediaItems.Get: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s=w%d-h%d", mediaItem.BaseURL, s.width, s.height), nil)
	if err != nil {
		return nil, fmt.Errorf("http.Get: %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do: %w", err)
	}

	defer res.Body.Close()

	img, _, err := image.Decode(res.Body)
	if err != nil {
		return nil, fmt.Errorf("image.Decode: %w", err)
	}

	return img, nil
}

func drawImage(fb *framebuffer.FrameBuffer, src image.Image) error {
	srcBounds := src.Bounds()
	frameBounds := fb.Bounds()

	converted := image.NewNRGBA(srcBounds)
	draw.Draw(converted, srcBounds, src, image.ZP, draw.Src)

	background := image.NewRGBA(frameBounds)
	draw.Draw(background, frameBounds, image.Black, image.ZP, draw.Src)

	resized := imaging.Resize(converted, frameBounds.Dx(), 0, imaging.Lanczos)
	resized = imaging.Fit(resized, frameBounds.Dx(), frameBounds.Dy(), imaging.Lanczos)
	final := imaging.PasteCenter(background, resized)

	draw.Draw(fb, frameBounds, final, image.ZP, draw.Src)
	return fb.Flush()
}

type Config struct {
	RotationInterval time.Duration
	Device           string
	Credentials      string
}

func Run(ctx context.Context, conf *Config) error {
	fb, err := framebuffer.Open(conf.Device)
	if err != nil {
		return fmt.Errorf("framebuffer.Open: %w", err)
	}
	defer fb.Close()

	credsRaw, err := os.ReadFile(conf.Credentials)
	if err != nil {
		return fmt.Errorf("os.ReadFile: %w", err)
	}

	oauth2Config, err := google.ConfigFromJSON(credsRaw, gphotos.PhotoslibraryReadonlyScope)
	if err != nil {
		return fmt.Errorf("google.CredentialsFromJSON: %w", err)
	}

	httpClient, err := getClient(ctx, oauth2Config)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}
	client, err := gphotos.NewClient(httpClient)
	if err != nil {
		return fmt.Errorf("gphotos.NewClient: %w", err)
	}

	group, ctx := errgroup.WithContext(ctx)

	frameWidth := uint16(fb.Bounds().Dx())
	frameHeight := uint16(fb.Bounds().Dy())

	slideshow := newSlideShow(client, frameWidth, frameHeight)

	group.Go(func() error {
		err := slideshow.loadLibrary(ctx)
		if err != nil {
			return fmt.Errorf("loadPhotoURLs: %w", err)
		}
		return nil
	})

	group.Go(func() error {
		timer := time.NewTicker(conf.RotationInterval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				image, err := slideshow.nextImage(ctx)
				if err != nil {
					log.Printf("slideshow.nextImage: %s\n", err)
					continue
				}

				if err := drawImage(fb, image); err != nil {
					log.Printf("drawImage: %s\n", err)
					continue
				}
			}
		}
	})

	return group.Wait()
}
