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

- For your arch:
```
go build
```

- For Raspberry Pie:

I didn't figure out how to cross-compile it:

`GOOS=linux GOARCH=arm GOARM=5 go build` fails to build the underlying `framebuffer` package.

I ended up installing golang in my rbp and compile it locally in there which took forever.
