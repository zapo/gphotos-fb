package main

import (
	"context"
	"flag"
	"gphotos-fb/gphotosfb"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/adrg/xdg"
)

func main() {
	sig := make(chan os.Signal, 1)
	defer close(sig)

	ctx, stop := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer stop()
	conf := gphotosfb.Config{}

	defaultCredentialsPath, err := xdg.ConfigFile("gphotos-fb/credentials.json")
	if err != nil {
		log.Fatalf("gphotos-fb: %s", err)
	}

	flag.StringVar(&conf.Device, "d", "/dev/fb0", "Path to framebuffer")
	flag.DurationVar(&conf.RotationInterval, "t", 10*time.Second, "Rotation interval")
	flag.StringVar(&conf.Credentials, "c", defaultCredentialsPath, "Oauth2 credentials")

	flag.Parse()

	if err := gphotosfb.Run(ctx, &conf); err != nil {
		log.Fatalf("gphotos-fb: %s", err)
	}
}
