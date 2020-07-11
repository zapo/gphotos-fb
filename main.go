package main

import (
	"context"
	"encoding/json"
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
	"os"
	"time"

	"github.com/adrg/xdg"
	"github.com/disintegration/imaging"
	"github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
	"github.com/zenhack/framebuffer-go"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) (client *http.Client, err error) {
	tokFile, err := xdg.CacheFile("gphotofb/token.json")
	if err != nil {
		return
	}

	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		err = saveToken(tokFile, tok)
		if err != nil {
			return
		}
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (tok *oauth2.Token, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			err = cerr
		}
	}()

	tok = &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) (err error) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return
	}

	defer func() {
		if cerr := f.Close(); cerr != nil {
			err = cerr
		}
	}()

	return json.NewEncoder(f).Encode(token)
}

func randomPhotoURL(ctx context.Context, client *http.Client) (url string, err error) {
	photoslibraryService, err := photoslibrary.New(client)
	if err != nil {
		return
	}

	var albums []*photoslibrary.Album
	err = photoslibraryService.Albums.List().Pages(ctx, func(res *photoslibrary.ListAlbumsResponse) error {
		albums = append(albums, res.Albums...)
		return nil
	})
	if err != nil {
		return
	}

	rand.Seed(time.Now().UTC().UnixNano())
	randAlbum := albums[rand.Intn(len(albums)-1)]

	if randAlbum.TotalMediaItems == 0 {
		return "", fmt.Errorf("Empty random album, retry")
	}

	searchCall := photoslibraryService.MediaItems.Search(
		&photoslibrary.SearchMediaItemsRequest{AlbumId: randAlbum.Id},
	)

	var items []*photoslibrary.MediaItem

	err = searchCall.Pages(ctx, func(res *photoslibrary.SearchMediaItemsResponse) (err error) {
		for _, item := range res.MediaItems {
			if item.MediaMetadata.Photo == nil {
				continue
			}
			items = append(items, item)
		}
		return
	})

	if err != nil {
		return
	}

	randItemIndex := rand.Intn(len(items) - 1)
	return items[randItemIndex].BaseUrl + "=w2048-h1024", nil
}

func drawImage(fb *framebuffer.FrameBuffer, src image.Image) error {
	b := src.Bounds()
	converted := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(converted, converted.Bounds(), src, b.Min, draw.Src)

	resized := imaging.Fill(converted, fb.Bounds().Max.X, fb.Bounds().Max.Y, imaging.Center, imaging.Lanczos)

	draw.Draw(fb, fb.Bounds(), resized, b.Bounds().Min, draw.Over)
	return fb.Flush()
}

func main() {
	var device, timeout, credsPath string
	flag.StringVar(&device, "d", "/dev/fb0", "Path to framebuffer")
	flag.StringVar(&timeout, "t", "10s", "Rotation timeout")
	flag.StringVar(&credsPath, "c", "./credentials.json", "Credentials path")
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

	client, err := getClient(config)
	if err != nil {
		log.Fatalf("Unable to initialize oauth client: %v", err)
	}

	for {
		func() {
			url, err := randomPhotoURL(ctx, client)
			defer func() {
				if err != nil {
					log.Println(err)
				}
			}()

			if err != nil {
				return
			}

			response, err := http.Get(url)
			if err != nil {
				return
			}

			defer func() {
				_ = response.Body.Close()
			}()

			image, _, err := image.Decode(response.Body)
			if err != nil {
				return
			}

			err = drawImage(fb, image)
			if err != nil {
				return
			}

			time.Sleep(duration)
		}()
	}
}
