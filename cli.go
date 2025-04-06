package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mdmcconnell/pdfcomp/pdfcomp"
)

func main() {
	iP := flag.Bool("images", false, "generate comparison images of pages that are different")
	pP := flag.Bool("pdf", false, "generate comparison images of pages that are different")
	rP := flag.Int("resolution", 300, "dpi resolution for comparison bitmaps")
	ratP := flag.Int("ratio", 30, "divide resolution by this  to determine the radius for difference outline circles")
	flag.Parse()
	fileArgs := flag.Args()
	images := *iP
	resolution := *rP
	ratio := *ratP
	pdf := *pP

	if len(fileArgs) != 2 {
		fmt.Fprintf(os.Stderr, "Wrong number of files give, need 2, received %d\n", len(fileArgs))
		printUse()
		os.Exit(2)
	}
	file1 := fileArgs[0]
	file2 := fileArgs[1]
	fmt.Printf("arguments received were images=%t, pdf=%t, radius=%d, resolution=%d, file1=%s, file2=%s\n", images, pdf, ratio, resolution, file1, file2)

	var w io.Writer
	if pdf {
		f, err := os.OpenFile(file1+"-diff.pdf", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err.Error())
			os.Exit(2)
		}
		w = f
		defer f.Close()
	}

	same, err := pdfcomp.EqualPDFs(file1, file2, images, w, resolution, ratio)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(2)
	}
	if same {
		os.Exit(0)
	}
	os.Exit(1)
}

func printUse() {
	fmt.Fprintf(os.Stderr, "usage: pdf-comp [-images -overwrite -radius=n -resolution=n] file1.pdf file2.pdf")
}
