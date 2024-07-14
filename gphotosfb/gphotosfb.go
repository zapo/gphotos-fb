package gphotosfb

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	gphotos "github.com/gphotosuploader/google-photos-api-client-go/v3"
	mediaitems "github.com/gphotosuploader/google-photos-api-client-go/v3/media_items"
	"github.com/zenhack/framebuffer-go"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
)

type urlList struct {
	mut  sync.RWMutex
	urls []string
	rng  *rand.Rand
}

func newURLList() *urlList {
	return &urlList{
		urls: make([]string, 0, 1024),
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (l *urlList) rand() (string, bool) {
	l.mut.RLock()
	defer l.mut.RUnlock()

	if len(l.urls) == 0 {
		return "", false
	}

	return l.urls[l.rng.Intn(len(l.urls)-1)], true
}

func (l *urlList) push(urls ...string) {
	l.mut.Lock()
	defer l.mut.Unlock()
	l.urls = append(l.urls, urls...)
}

func loadPhotoURLs(ctx context.Context, client *http.Client, urlList *urlList) error {
	photoslibraryService, err := gphotos.NewClient(client)
	if err != nil {
		return fmt.Errorf("gphotos.NewClient: %w", err)
	}

	var pageToken string
	for {
		items, nextToken, err := photoslibraryService.MediaItems.PaginatedList(ctx, &mediaitems.PaginatedListOptions{
			PageToken: pageToken,
		})
		if err != nil {
			return fmt.Errorf("photoslibraryService.MediaItems.List: %w", err)
		}
		if nextToken == "" {
			break
		}
		pageToken = nextToken

		urls := make([]string, len(items))
		for i, item := range items {
			urls[i] = item.BaseURL
		}
		urlList.push(urls...)
	}

	return nil
}

func fetchImage(ctx context.Context, url string, width, height uint16) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s=w%d-h%d", url, width, height), nil)
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

func display(ctx context.Context, fb *framebuffer.FrameBuffer, url string) error {
	frameWidth := uint16(fb.Bounds().Dx())
	frameHeight := uint16(fb.Bounds().Dy())

	image, err := fetchImage(ctx, url, frameWidth, frameHeight)
	if err != nil {
		return fmt.Errorf("fetchImage(%s): %w", url, err)
	}

	if err := drawImage(fb, image); err != nil {
		return fmt.Errorf("drawImage(%s): %w", url, err)
	}
	return nil
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

	client, err := getClient(ctx, oauth2Config)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}

	group, ctx := errgroup.WithContext(ctx)
	urls := newURLList()

	group.Go(func() error {
		err := loadPhotoURLs(ctx, client, urls)
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
				url, ok := urls.rand()
				if !ok {
					continue
				}

				if err := display(ctx, fb, url); err != nil {
					fmt.Printf("display: %s\n", err)
					continue
				}
			}
		}
	})

	return group.Wait()
}
