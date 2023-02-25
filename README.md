# gphotos-fb

Linux Framebuffer display of a random google photo picture.

Trying to make my Raspberry Pie useful!

## Usage

```
$ gphotos-fb --help
Usage of ./gphotos-fb:
  -c string
        Credentials path (default "$HOME/.config/gphotos-fb/credentials.json")
  -d string
        Path to framebuffer (default "/dev/fb0")
  -t string
        Rotation interval (default "10s")
```

## Build

```
make
```

or

```
go build -o gphotos-fb cmd/main.go
```
