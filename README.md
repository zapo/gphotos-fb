# gphotofb
Linux Framebuffer display of random google photo picture.

Trying to make my Raspberry Pie useful!

## Usage
```
$ gphotofb --help
Usage of ./gphotofb:
  -c string
        Credentials path (default "./credentials.json")
  -d string
        Path to framebuffer (default "/dev/fb0")
  -t string
        Rotation timeout (default "10s")
```

## Build

```
go build
```

Note that I didn't figure out how to cross-compile it for raspberry pie.
`GOOS=linux GOARCH=arm GOARM=5 go build` fails to build the underlying `framebuffer` package.

I ended up installing go on my rbp and compile it in there which took forever.
