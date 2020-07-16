package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"gphotofb/internal/auth"

	"github.com/adrg/xdg"
	"github.com/disintegration/imaging"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/zenhack/framebuffer-go"
	"golang.org/x/oauth2/google"
)

func loadPhotoURLs(ctx context.Context, client *http.Client, urls chan string) (err error) {
	photoslibraryService, err := photoslibrary.New(client)
	if err != nil {
		return
	}

	searchCall := photoslibraryService.MediaItems.Search(
		&photoslibrary.SearchMediaItemsRequest{
			PageSize: 50,
			Filters: &photoslibrary.Filters{
				MediaTypeFilter: &photoslibrary.MediaTypeFilter{
					MediaTypes: []string{"PHOTO"},
				},
			},
		},
	)

	err = searchCall.Pages(ctx, func(res *photoslibrary.SearchMediaItemsResponse) (err error) {
		for _, item := range res.MediaItems {
			urls <- item.BaseUrl
		}
		return nil
	})

	return
}

func fetchImage(url string, width uint16, height uint16) (img image.Image, err error) {
	response, err := http.Get(fmt.Sprintf("%s=w%d-h%d", url, width, height))
	if err != nil {
		return
	}

	defer func() {
		if cerr := response.Body.Close(); err != nil {
			err = cerr
		}
	}()

	img, _, err = image.Decode(response.Body)
	if err != nil {
		return
	}
	return
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

func main() {

	var device, timeout, credsPath string
	flag.StringVar(&device, "d", "/dev/fb0", "Path to framebuffer")
	flag.StringVar(&timeout, "t", "10s", "Rotation timeout")
	defaultCreds, err := xdg.ConfigFile("gphotofb/credentials.json")
	if err != nil {
		defaultCreds = "./credentials.json"
	}
	flag.StringVar(&credsPath, "c", defaultCreds, "Credentials path")
	flag.Parse()

	duration, err := time.ParseDuration(timeout)
	if err != nil {
		log.Fatalf("Unable to parse TIMEOUT duration: %v", err)
	}

	fb, err := framebuffer.Open(device)
	if err != nil {
		log.Fatalf("Unable to initialize framebuffer: %v", err)
	}
	defer func() {
		if err := fb.Close(); err != nil {
			log.Fatalf("Unable to close framebuffer: %v", err)
		}
	}()

	ctx := context.Background()
	b, err := ioutil.ReadFile(credsPath)

	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	config, err := google.ConfigFromJSON(b, photoslibrary.PhotoslibraryReadonlyScope)
	if err != nil {
		log.Fatal(err)
	}

	client, err := auth.GetClient(ctx, config)
	if err != nil {
		log.Fatalf("Unable to initialize oauth client: %v", err)
	}

	rand.Seed(time.Now().UTC().UnixNano())

	urlStream := make(chan string)
	show := make(chan struct{})

	go func() {
		err := loadPhotoURLs(ctx, client, urlStream)
		if err != nil {
			log.Fatalf("Unable to get photo library list: %v", err)
		}
	}()

	go func() {
		for {
			time.Sleep(duration)
			show <- struct{}{}
		}
	}()

	urls := []string{}
	frameWidth := fb.Bounds().Dx()
	frameHeight := fb.Bounds().Dy()

	log.Printf("Drawing on %dx%d surface\n", frameWidth, frameHeight)

	for {
		select {
		case <-show:
			if len(urls) == 0 {
				log.Println("Empty collection, skipping ...")
				continue
			}

			log.Printf("Displaying random picture (total %d)\n", len(urls))

			url := urls[rand.Intn(len(urls))]
			image, err := fetchImage(url, uint16(frameWidth), uint16(frameHeight))
			if err != nil || image == nil {
				log.Printf("Unable to load photo at %s: %v\n", url, err)
			}

			err = drawImage(fb, image)
			if err != nil {
				log.Printf("Unable to draw photo at %s: %v\n", url, err)
			}
		case url := <-urlStream:
			urls = append(urls, url)
		}
	}
}
