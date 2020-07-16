# gphotofb
Linux Framebuffer display of random google photo picture.

Trying to make my Raspberry Pie useful!

## Usage
```
$ gphotofb --help
Usage of ./gphotofb:
  -c string
        Credentials path (default "$HOME/.config/gphotofb/credentials.json")
  -d string
        Path to framebuffer (default "/dev/fb0")
  -t string
        Rotation timeout (default "10s")
```

## Build

```
make
```

or

```
go build -o gphotofb cmd/main.go
```
