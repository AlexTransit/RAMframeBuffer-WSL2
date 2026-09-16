package main

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	fbPath = "/dev/fb0"

	width  = 240
	height = 320
	bpp    = 2

	fbSize = width * height * bpp
	scale  = 2
)

func rgb565ToRGBA(src []byte, dst []byte) {
	for i := 0; i < width*height; i++ {
		p := uint16(src[i*2]) | uint16(src[i*2+1])<<8

		r := uint8((p >> 11) & 0x1f)
		g := uint8((p >> 5) & 0x3f)
		b := uint8(p & 0x1f)

		// RGB565 -> 8 bit
		r = (r << 3) | (r >> 2)
		g = (g << 2) | (g >> 4)
		b = (b << 3) | (b >> 2)

		j := i * 4

		dst[j+0] = r
		dst[j+1] = g
		dst[j+2] = b
		dst[j+3] = 0xff
	}
}

func main() {
	fb, err := os.OpenFile(fbPath, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", fbPath, err)
		os.Exit(1)
	}
	defer fb.Close()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		panic(err)
	}
	defer sdl.Quit()

window, err := sdl.CreateWindow(
	"VMC framebuffer",
	sdl.WINDOWPOS_CENTERED,
	sdl.WINDOWPOS_CENTERED,
	width*scale,
	height*scale,
	sdl.WINDOW_SHOWN,
)
if err != nil {
	panic(err)
}
defer window.Destroy()

renderer, err := sdl.CreateRenderer(
	window,
	-1,
	sdl.RENDERER_ACCELERATED,
)
if err != nil {
	// Для WSL попробуем software renderer.
	renderer, err = sdl.CreateRenderer(
		window,
		-1,
		sdl.RENDERER_SOFTWARE,
	)
	if err != nil {
		panic(err)
	}
}
defer renderer.Destroy()

	texture, err := renderer.CreateTexture(
		sdl.PIXELFORMAT_ABGR8888,
		sdl.TEXTUREACCESS_STREAMING,
		width,
		height,
	)
	if err != nil {
		panic(err)
	}
	defer texture.Destroy()

	src := make([]byte, fbSize)
	rgba := make([]byte, width*height*4)

	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	running := true

	for running {
		// Обрабатываем события окна.
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				running = false
			}
		}

		if !running {
			break
		}

		select {
		case <-ticker.C:
			if _, err := fb.Seek(0, 0); err != nil {
				fmt.Fprintf(os.Stderr, "seek framebuffer: %v\n", err)
				continue
			}

			n, err := fb.Read(src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "read framebuffer: %v\n", err)
				continue
			}

			if n != fbSize {
				fmt.Fprintf(
					os.Stderr,
					"short framebuffer read: %d/%d\n",
					n,
					fbSize,
				)
				continue
			}

			rgb565ToRGBA(src, rgba)

			if err := texture.Update(
				nil,
				unsafe.Pointer(&rgba[0]),
				width*4,
			); err != nil {
				fmt.Fprintf(os.Stderr, "texture update: %v\n", err)
				continue
			}

			renderer.Clear()

			if err := renderer.Copy(texture, nil, nil); err != nil {
				fmt.Fprintf(os.Stderr, "renderer copy: %v\n", err)
				continue
			}

			renderer.Present()

		default:
			time.Sleep(time.Millisecond)
		}
	}
}

